package synthetictests

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	v1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/openshift/origin/pkg/duplicateevents"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	duplicateEventThreshold           = 20
	duplicateSingleNodeEventThreshold = 30
)

var allowedRepeatedEventFns = []isRepeatedEventOKFunc{
	isConsoleReadinessDuringInstallation,
	isConfigOperatorReadinessFailed,
	isConfigOperatorProbeErrorReadinessFailed,
	isConfigOperatorProbeErrorLivenessFailed,
	isOauthApiserverProbeErrorReadinessFailed,
	isOauthApiserverProbeErrorLivenessFailed,
	isOauthApiserverProbeErrorConnectionRefusedFailed,
}

var allowedSingleNodeRepeatedEventFns = []isRepeatedEventOKFunc{
	isConnectionRefusedOnSingleNode,
}

var knownEventsBugs = []knownProblem{
	{
		Regexp: regexp.MustCompile(`ns/openshift-multus pod/network-metrics-daemon-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-e2e-loki pod/loki-promtail-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-network-diagnostics pod/network-check-target-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
	{
		Regexp: regexp.MustCompile(`ns/.* service/.* - reason/FailedToDeleteOVNLoadBalancer .*`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1990631",
	},
	{
		Regexp: regexp.MustCompile(`ns/.*horizontalpodautoscaler.*failed to get cpu utilization: unable to get metrics for resource cpu: no metrics returned from resource metrics API.*`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1993985",
	},
	{
		Regexp: regexp.MustCompile(`ns/.*unable to ensure pod container exists: failed to create container.*slice already exists.*`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1993980",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-etcd pod/etcd-quorum-guard-[a-z0-9-]+ node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=2000234",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-etcd pod/etcd-guard-.* node/.* - reason/ProbeError Readiness probe error: .* connect: connection refused`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=2075204",
	},
	{
		Regexp: regexp.MustCompile("ns/openshift-etcd-operator namespace/openshift-etcd-operator -.*rpc error: code = Canceled desc = grpc: the client connection is closing.*"),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=2006975",
	},
	{
		Regexp: regexp.MustCompile("reason/TopologyAwareHintsDisabled"),
		BZ:     "https://issues.redhat.com/browse/OCPBUGS-5943",
	},
	{
		Regexp:   regexp.MustCompile("ns/.*reason/.*APICheckFailed.*503.*"),
		BZ:       "https://bugzilla.redhat.com/show_bug.cgi?id=2017435",
		Topology: topologyPointer(v1.SingleReplicaTopologyMode),
	},
	// builds tests trigger many changes in the config which creates new rollouts -> event for each pod
	// working as intended (not a bug) and needs to be tolerated
	{
		Regexp:    regexp.MustCompile(`ns/openshift-route-controller-manager deployment/route-controller-manager - reason/ScalingReplicaSet \(combined from similar events\): Scaled (down|up) replica set route-controller-manager-[a-z0-9-]+ to [0-9]+`),
		TestSuite: stringPointer("openshift/build"),
	},
	// builds tests trigger many changes in the config which creates new rollouts -> event for each pod
	// working as intended (not a bug) and needs to be tolerated
	{
		Regexp:    regexp.MustCompile(`ns/openshift-controller-manager deployment/controller-manager - reason/ScalingReplicaSet \(combined from similar events\): Scaled (down|up) replica set controller-manager-[a-z0-9-]+ to [0-9]+`),
		TestSuite: stringPointer("openshift/build"),
	},
	//{ TODO this should only be skipped for single-node
	//	name:    "single=node-storage",
	//  BZ: https://bugzilla.redhat.com/show_bug.cgi?id=1990662
	//	message: "ns/openshift-cluster-csi-drivers pod/aws-ebs-csi-driver-controller-66469455cd-2thfv node/ip-10-0-161-38.us-east-2.compute.internal - reason/BackOff Back-off restarting failed container",
	//},
}

type duplicateEventsEvaluator struct {
	allowedRepeatedEventPatterns []*regexp.Regexp
	allowedRepeatedEventFns      []isRepeatedEventOKFunc

	// knownRepeatedEventsBugs are duplicates that are considered bugs and should flake, but not  fail a test
	knownRepeatedEventsBugs []knownProblem

	// platform contains the current platform of the cluster under test.
	platform v1.PlatformType

	// topology contains the topology of the cluster under test.
	topology v1.TopologyMode

	// testSuite contains the name of the test suite invoked.
	testSuite string
}

type knownProblem struct {
	Regexp *regexp.Regexp
	BZ     string

	// Platform limits the exception to a specific OpenShift platform.
	Platform *v1.PlatformType

	// Topology limits the exception to a specific topology (e.g. single replica)
	Topology *v1.TopologyMode

	// TestSuite limits the exception to a specific test suite (e.g. openshift/builds)
	TestSuite *string
}

func testDuplicatedEventForUpgrade(events monitorapi.Intervals, kubeClientConfig *rest.Config, testSuite string) []*junitapi.JUnitTestCase {
	allowedPatterns := []*regexp.Regexp{}
	allowedPatterns = append(allowedPatterns, duplicateevents.AllowedRepeatedEventPatterns...)
	allowedPatterns = append(allowedPatterns, duplicateevents.AllowedUpgradeRepeatedEventPatterns...)

	evaluator := duplicateEventsEvaluator{
		allowedRepeatedEventPatterns: allowedPatterns,
		allowedRepeatedEventFns:      allowedRepeatedEventFns,
		knownRepeatedEventsBugs:      knownEventsBugs,
		testSuite:                    testSuite,
	}

	if err := evaluator.getClusterInfo(kubeClientConfig); err != nil {
		e2e.Logf("could not fetch cluster info: %w", err)
	}

	if evaluator.topology == v1.SingleReplicaTopologyMode {
		evaluator.allowedRepeatedEventFns = append(evaluator.allowedRepeatedEventFns, allowedSingleNodeRepeatedEventFns...)
	}

	tests := []*junitapi.JUnitTestCase{}
	tests = append(tests, evaluator.testDuplicatedCoreNamespaceEvents(events, kubeClientConfig)...)
	tests = append(tests, evaluator.testDuplicatedE2ENamespaceEvents(events, kubeClientConfig)...)
	return tests
}

func testDuplicatedEventForStableSystem(events monitorapi.Intervals, clientConfig *rest.Config, testSuite string) []*junitapi.JUnitTestCase {
	evaluator := duplicateEventsEvaluator{
		allowedRepeatedEventPatterns: duplicateevents.AllowedRepeatedEventPatterns,
		allowedRepeatedEventFns:      allowedRepeatedEventFns,
		knownRepeatedEventsBugs:      knownEventsBugs,
		testSuite:                    testSuite,
	}

	operatorClient, err := operatorv1client.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}
	etcdAllowance, err := newDuplicatedEventsAllowedWhenEtcdRevisionChange(context.TODO(), operatorClient)
	if err != nil {
		panic(fmt.Errorf("unable to construct duplicated events allowance for etcd, err = %v", err))
	}
	evaluator.allowedRepeatedEventFns = append(evaluator.allowedRepeatedEventFns, etcdAllowance.allowEtcdGuardReadinessProbeFailure)

	if err := evaluator.getClusterInfo(clientConfig); err != nil {
		e2e.Logf("could not fetch cluster info: %w", err)
	}

	if evaluator.topology == v1.SingleReplicaTopologyMode {
		evaluator.allowedRepeatedEventFns = append(evaluator.allowedRepeatedEventFns, allowedSingleNodeRepeatedEventFns...)
	}

	tests := []*junitapi.JUnitTestCase{}
	tests = append(tests, evaluator.testDuplicatedCoreNamespaceEvents(events, clientConfig)...)
	tests = append(tests, evaluator.testDuplicatedE2ENamespaceEvents(events, clientConfig)...)
	return tests
}

// isRepeatedEventOKFunc takes a monitorEvent as input and returns true if the repeated event is OK.
// This commonly happens for known bugs and for cases where events are repeated intentionally by tests.
// Use this to handle cases where, "if X is true, then the repeated event is ok".
type isRepeatedEventOKFunc func(monitorEvent monitorapi.EventInterval, kubeClientConfig *rest.Config, times int) (bool, error)

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedCoreNamespaceEvents(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[sig-arch] events should not repeat pathologically"

	return d.testDuplicatedEvents(testName, false, events.Filter(monitorapi.Not(monitorapi.IsInE2ENamespace)), kubeClientConfig)
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedE2ENamespaceEvents(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[sig-arch] events should not repeat pathologically in e2e namespaces"

	return d.testDuplicatedEvents(testName, true, events.Filter(monitorapi.IsInE2ENamespace), kubeClientConfig)
}

// appendToFirstLine appends add to the end of the first line of s
func appendToFirstLine(s string, add string) string {
	splits := strings.Split(s, "\n")
	splits[0] += add
	return strings.Join(splits, "\n")
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedEvents(testName string, flakeOnly bool, events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	allowedRepeatedEventsRegex := duplicateevents.CombinedRegexp(d.allowedRepeatedEventPatterns...)

	type pathologicalEvents struct {
		count        int
		eventMessage string
		from         time.Time
		to           time.Time
	}

	var failures []string
	displayToCount := map[string]*pathologicalEvents{}
	for _, event := range events {
		eventDisplayMessage, times := getTimesAnEventHappened(fmt.Sprintf("%s - %s", event.Locator, event.Message))
		if times > duplicateEventThreshold {
			if allowedRepeatedEventsRegex.MatchString(eventDisplayMessage) {
				continue
			}

			allowed := false
			for _, allowRepeatedEventFn := range d.allowedRepeatedEventFns {
				var err error
				allowed, err = allowRepeatedEventFn(event, kubeClientConfig, times)
				if err != nil {
					failures = append(failures, fmt.Sprintf("error: [%v] when processing event %v", err, eventDisplayMessage))
					allowed = false
					continue
				}
				if allowed {
					break
				}
			}
			if allowed {
				continue
			}

			eventMessageString := eventDisplayMessage + " From: " + event.From.Format("15:04:05Z") + " To: " + event.To.Format("15:04:05Z")
			if _, ok := displayToCount[eventMessageString]; !ok {
				tmp := &pathologicalEvents{
					count:        times,
					eventMessage: eventDisplayMessage,
					from:         event.From,
					to:           event.To,
				}
				displayToCount[eventMessageString] = tmp
			}
			if times > displayToCount[eventMessageString].count {
				displayToCount[eventMessageString].count = times
			}
		}
	}

	var flakes []string
	for msgWithTime, pathoItem := range displayToCount {
		msg := fmt.Sprintf("event happened %d times, something is wrong: %v", pathoItem.count, msgWithTime)
		flake := false
		for _, kp := range d.knownRepeatedEventsBugs {
			if kp.Regexp != nil && kp.Regexp.MatchString(pathoItem.eventMessage) {
				// Check if this exception only applies to our specific platform
				if kp.Platform != nil && *kp.Platform != d.platform {
					continue
				}

				// Check if this exception only applies to a specific topology
				if kp.Topology != nil && *kp.Topology != d.topology {
					continue
				}

				// Check if this exception only applies to a specific test suite
				if kp.TestSuite != nil && *kp.TestSuite != d.testSuite {
					continue
				}

				msg += " - " + kp.BZ
				flake = true
			}
		}

		if flake || flakeOnly {
			flakes = append(flakes, appendToFirstLine(msg, " result=allow "))
		} else {
			failures = append(failures, appendToFirstLine(msg, " result=reject "))
		}
	}

	// failures during a run always fail the test suite
	var tests []*junitapi.JUnitTestCase
	if len(failures) > 0 || len(flakes) > 0 {
		var output string
		if len(failures) > 0 {
			output = fmt.Sprintf("%d events happened too frequently\n\n%v", len(failures), strings.Join(failures, "\n"))
		}
		if len(flakes) > 0 {
			if output != "" {
				output += "\n\n"
			}
			output += fmt.Sprintf("%d events with known BZs\n\n%v", len(flakes), strings.Join(flakes, "\n"))
		}
		tests = append(tests, &junitapi.JUnitTestCase{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: output,
			},
		})
	}

	if len(tests) == 0 || len(failures) == 0 {
		// Add a successful result to mark the test as flaky if there are no
		// unknown problems.
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	}
	return tests
}

var eventCountExtractor = regexp.MustCompile(`(?s)(.*) \((\d+) times\).*`)

func getTimesAnEventHappened(message string) (string, int) {
	matches := eventCountExtractor.FindAllStringSubmatch(message, -1)
	if len(matches) != 1 { // not present or weird
		return "", 0
	}
	if len(matches[0]) < 2 { // no capture
		return "", 0
	}
	times, err := strconv.ParseInt(matches[0][2], 10, 0)
	if err != nil { // not an int somehow
		return "", 0
	}
	return matches[0][1], int(times)
}

func getInstallCompletionTime(kubeClientConfig *rest.Config) *metav1.Time {
	configClient, err := configclient.NewForConfig(kubeClientConfig)
	if err != nil {
		return nil
	}
	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		return nil
	}
	if len(clusterVersion.Status.History) == 0 {
		return nil
	}
	return clusterVersion.Status.History[len(clusterVersion.Status.History)-1].CompletionTime
}

func getMatchedElementsFromMonitorEventMsg(regExp *regexp.Regexp, message string) (string, string, string, string, string, error) {
	var namespace, pod, node, reason, msg string
	if !regExp.MatchString(message) {
		return namespace, pod, node, reason, msg, errors.New("regex match error")
	}
	subMatches := regExp.FindStringSubmatch(message)
	subNames := regExp.SubexpNames()
	for i, name := range subNames {
		switch name {
		case "NS":
			namespace = subMatches[i]
		case "POD":
			pod = subMatches[i]
		case "NODE":
			node = subMatches[i]
		case "REASON":
			reason = subMatches[i]
		case "MSG":
			msg = subMatches[i]
		}
	}
	if len(namespace) == 0 ||
		len(pod) == 0 ||
		len(node) == 0 ||
		len(msg) == 0 {
		return namespace, pod, node, reason, msg, fmt.Errorf("regex match expects non-empty elements, got namespace: %s, pod: %s, node: %s, msg: %s", namespace, pod, node, msg)
	}
	return namespace, pod, node, reason, msg, nil
}

// isEventDuringInstallation returns true if the monitorEvent represents a real event that happened after installation.
// regExp defines the pattern of the monitorEvent message. Named match is used in the pattern using `(?P<>)`. The names are placed inside <>. See example below
// `ns/(?P<NS>openshift-ovn-kubernetes) pod/(?P<POD>ovnkube-node-[a-z0-9-]+) node/(?P<NODE>[a-z0-9.-]+) - reason/(?P<REASON>Unhealthy) (?P<MSG>Readiness probe failed:.*$`
func isEventDuringInstallation(monitorEvent monitorapi.EventInterval, kubeClientConfig *rest.Config, regExp *regexp.Regexp) (bool, error) {
	if kubeClientConfig == nil {
		// default to OK
		return true, nil
	}
	installCompletionTime := getInstallCompletionTime(kubeClientConfig)
	if installCompletionTime == nil {
		return true, nil
	}

	message := fmt.Sprintf("%s - %s", monitorEvent.Locator, monitorEvent.Message)
	namespace, pod, _, reason, msg, err := getMatchedElementsFromMonitorEventMsg(regExp, message)
	if err != nil {
		return false, err
	}
	kubeClient, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return true, nil
	}
	kubeEvents, err := kubeClient.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return true, nil
	}
	for _, event := range kubeEvents.Items {
		if event.Related == nil ||
			event.Related.Name != pod ||
			event.Reason != reason ||
			!strings.Contains(event.Message, msg) {
			continue
		}

		if event.FirstTimestamp.After(installCompletionTime.Time) {
			return false, nil
		}
	}
	return true, nil
}

// isConsoleReadinessDuringInstallation returns true if the event is for console readiness and it happens during the
// initial installation of the cluster.
// we're looking for something like
// > ns/openshift-console pod/console-7c6f797fd9-5m94j node/ip-10-0-158-106.us-west-2.compute.internal - reason/ProbeError Readiness probe error: Get "https://10.129.0.49:8443/health": dial tcp 10.129.0.49:8443: connect: connection refused
// with a firstTimestamp before the cluster completed the initial installation
func isConsoleReadinessDuringInstallation(monitorEvent monitorapi.EventInterval, kubeClientConfig *rest.Config, _ int) (bool, error) {
	if !strings.Contains(monitorEvent.Locator, "ns/openshift-console") {
		return false, nil
	}
	if !strings.Contains(monitorEvent.Locator, "pod/console-") {
		return false, nil
	}
	if !strings.Contains(monitorEvent.Locator, "Readiness probe") {
		return false, nil
	}
	if !strings.Contains(monitorEvent.Locator, "connect: connection refused") {
		return false, nil
	}

	regExp := regexp.MustCompile(duplicateevents.ConsoleReadinessRegExpStr)
	// if the readiness probe failure for this pod happened AFTER the initial installation was complete,
	// then this probe failure is unexpected and should fail.
	return isEventDuringInstallation(monitorEvent, kubeClientConfig, regExp)
}

// isConfigOperatorReadinessFailed returns true if the event matches a readinessFailed error that timed out
// in the openshift-config-operator.
// like this:
// ...ReadinessFailed Get \"https://10.130.0.16:8443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
func isConfigOperatorReadinessFailed(monitorEvent monitorapi.EventInterval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(readinessFailedMessageRegExpStr)
	return isOperatorMatchRegexMessage(monitorEvent, "openshift-config-operator", regExp), nil
}

// isConfigOperatorProbeErrorReadinessFailed returns true if the event matches a ProbeError Readiness Probe message
// in the openshift-config-operator.
// like this:
// reason/ProbeError Readiness probe error: Get "https://10.130.0.15:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
func isConfigOperatorProbeErrorReadinessFailed(monitorEvent monitorapi.EventInterval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(probeErrorReadinessMessageRegExpStr)
	return isOperatorMatchRegexMessage(monitorEvent, "openshift-config-operator", regExp), nil
}

// isConfigOperatorProbeErrorLivenessFailed returns true if the event matches a ProbeError Liveness Probe message
// in the openshift-config-operator.
// like this:
// ...reason/ProbeError Liveness probe error: Get "https://10.128.0.21:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
func isConfigOperatorProbeErrorLivenessFailed(monitorEvent monitorapi.EventInterval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(probeErrorLivenessMessageRegExpStr)
	return isOperatorMatchRegexMessage(monitorEvent, "openshift-config-operator", regExp), nil
}

// isOauthApiserverProbeErrorReadinessFailed returns true if the event matches a ProbeError Readiness Probe message
// in the openshift-oauth-operator.
// like this:
// ...ns/openshift-oauth-apiserver pod/apiserver-65fd7ffc59-bt5sf node/q72hs3bx-ac890-4pxpm-master-2 - reason/ProbeError Readiness probe error: Get "https://10.129.0.8:8443/readyz": net/http: request canceled (Client.Timeout exceeded while awaiting headers)
func isOauthApiserverProbeErrorReadinessFailed(monitorEvent monitorapi.EventInterval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(probeErrorReadinessMessageRegExpStr)
	return isOperatorMatchRegexMessage(monitorEvent, "openshift-oauth-apiserver", regExp), nil
}

// isOauthApiserverProbeErrorLivenessFailed returns true if the event matches a ProbeError Liveness Probe message
// in the openshift-oauth-operator.
// like this:
// ...reason/ProbeError Liveness probe error: Get "https://10.130.0.68:8443/healthz": net/http: request canceled (Client.Timeout exceeded while awaiting headers)
func isOauthApiserverProbeErrorLivenessFailed(monitorEvent monitorapi.EventInterval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(probeErrorLivenessMessageRegExpStr)
	return isOperatorMatchRegexMessage(monitorEvent, "openshift-oauth-apiserver", regExp), nil
}

// isOauthApiserverProbeErrorConnectionRefusedFailed returns true if the event matches a ProbeError Readiness Probe connection refused message
// in the openshift-oauth-operator.
// like this:
// ...ns/openshift-oauth-apiserver pod/apiserver-647fc6c7bf-s8b4h node/ip-10-0-150-209.us-west-1.compute.internal - reason/ProbeError Readiness probe error: Get "https://10.128.0.38:8443/readyz": dial tcp 10.128.0.38:8443: connect: connection refused
func isOauthApiserverProbeErrorConnectionRefusedFailed(monitorEvent monitorapi.EventInterval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(probeErrorConnectionRefusedRegExpStr)
	return isOperatorMatchRegexMessage(monitorEvent, "openshift-oauth-apiserver", regExp), nil
}

// isConnectionRefusedOnSingleNode returns true if the event matched has a connection refused message for single node events and is with in threshold.
func isConnectionRefusedOnSingleNode(monitorEvent monitorapi.EventInterval, _ *rest.Config, count int) (bool, error) {
	regExp := regexp.MustCompile(singleNodeErrorConnectionRefusedRegExpStr)
	return regExp.MatchString(monitorEvent.String()) && count < duplicateSingleNodeEventThreshold, nil
}

// isOperatorMatchRegexMessage returns true if this monitorEvent is for the operator identified by the operatorName
// and its message matches the given regex.
func isOperatorMatchRegexMessage(monitorEvent monitorapi.EventInterval, operatorName string, regExp *regexp.Regexp) bool {
	locatorParts := monitorapi.LocatorParts(monitorEvent.Locator)
	if ns, ok := locatorParts["ns"]; ok {
		if ns != operatorName {
			return false
		}
	}
	if pod, ok := locatorParts["pod"]; ok {
		if !strings.HasPrefix(pod, operatorName) {
			return false
		}
	}
	if !regExp.MatchString(monitorEvent.Message) {
		return false
	}
	return true
}

func (d *duplicateEventsEvaluator) getClusterInfo(c *rest.Config) (err error) {
	if c == nil {
		return
	}

	oc, err := configclient.NewForConfig(c)
	if err != nil {
		return err
	}
	infra, err := oc.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}

	if infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.Type != "" {
		d.platform = infra.Status.PlatformStatus.Type
	}

	if infra.Status.ControlPlaneTopology != "" {
		d.topology = infra.Status.ControlPlaneTopology
	}

	return nil
}

func topologyPointer(topology v1.TopologyMode) *v1.TopologyMode {
	return &topology
}

func platformPointer(platform v1.PlatformType) *v1.PlatformType {
	return &platform
}

func stringPointer(testSuite string) *string {
	return &testSuite
}

type etcdRevisionChangeAllowance struct {
	allowedGuardProbeFailurePattern        *regexp.Regexp
	maxAllowedGuardProbeFailurePerRevision int

	currentRevision int
}

func newDuplicatedEventsAllowedWhenEtcdRevisionChange(ctx context.Context, operatorClient operatorv1client.OperatorV1Interface) (*etcdRevisionChangeAllowance, error) {
	currentRevision, err := getBiggestRevisionForEtcdOperator(ctx, operatorClient)
	if err != nil {
		return nil, err
	}
	return &etcdRevisionChangeAllowance{
		allowedGuardProbeFailurePattern:        regexp.MustCompile(`ns/openshift-etcd pod/etcd-guard-.* node/[a-z0-9.-]+ - reason/(Unhealthy|ProbeError) Readiness probe.*`),
		maxAllowedGuardProbeFailurePerRevision: 60 / 5, // 60s for starting a new pod, divided by the probe interval
		currentRevision:                        currentRevision,
	}, nil
}

// allowEtcdGuardReadinessProbeFailure tolerates events that match allowedGuardProbeFailurePattern unless we receive more than a.maxAllowedGuardProbeFailurePerRevision*a.currentRevision
func (a *etcdRevisionChangeAllowance) allowEtcdGuardReadinessProbeFailure(monitorEvent monitorapi.EventInterval, _ *rest.Config, times int) (bool, error) {
	eventMessage := fmt.Sprintf("%s - %s", monitorEvent.Locator, monitorEvent.Message)

	// allow for a.maxAllowedGuardProbeFailurePerRevision * a.currentRevision failed readiness probe from the etcd-guard pods
	// since the guards are static and the etcd pods come and go during a rollout
	// which causes allowedGuardProbeFailurePattern to fire
	if a.allowedGuardProbeFailurePattern.MatchString(eventMessage) && a.maxAllowedGuardProbeFailurePerRevision*a.currentRevision > times {
		return true, nil
	}
	return false, nil
}

// getBiggestRevisionForEtcdOperator calculates the biggest revision among replicas of the most recently successful deployment
func getBiggestRevisionForEtcdOperator(ctx context.Context, operatorClient operatorv1client.OperatorV1Interface) (int, error) {
	etcd, err := operatorClient.Etcds().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		// instead of panicking when there no etcd operator (e.g. microshift), just estimate the biggest revision to be 0
		if apierrors.IsNotFound(err) {
			return 0, nil
		} else {
			return 0, err
		}

	}
	biggestRevision := 0
	for _, nodeStatus := range etcd.Status.NodeStatuses {
		if int(nodeStatus.CurrentRevision) > biggestRevision {
			biggestRevision = int(nodeStatus.CurrentRevision)
		}
	}
	return biggestRevision, nil
}
