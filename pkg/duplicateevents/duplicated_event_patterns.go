package duplicateevents

import (
	"regexp"
)

const (
	ImagePullRedhatRegEx             = `reason/[a-zA-Z]+ .*Back-off pulling image .*registry.redhat.io`
	RequiredResourcesMissingRegEx    = `reason/RequiredInstallerResourcesMissing secrets: etcd-all-certs-[0-9]+`
	BackoffRestartingFailedRegEx     = `reason/BackOff Back-off restarting failed container`
	ErrorUpdatingEndpointSlicesRegex = `reason/FailedToUpdateEndpointSlices Error updating Endpoint Slices`
	NodeHasNoDiskPressureRegExpStr   = "reason/NodeHasNoDiskPressure.*status is now: NodeHasNoDiskPressure"
	NodeHasSufficientMemoryRegExpStr = "reason/NodeHasSufficientMemory.*status is now: NodeHasSufficientMemory"
	NodeHasSufficientPIDRegExpStr    = "reason/NodeHasSufficientPID.*status is now: NodeHasSufficientPID"

	OvnReadinessRegExpStr                   = `ns/(?P<NS>openshift-ovn-kubernetes) pod/(?P<POD>ovnkube-node-[a-z0-9-]+) node/(?P<NODE>[a-z0-9.-]+) - reason/(?P<REASON>Unhealthy) (?P<MSG>Readiness probe failed:.*$)`
	ConsoleReadinessRegExpStr               = `ns/(?P<NS>openshift-console) pod/(?P<POD>console-[a-z0-9-]+) node/(?P<NODE>[a-z0-9.-]+) - reason/(?P<REASON>ProbeError) (?P<MSG>Readiness probe error:.* connect: connection refused$)`
	MarketplaceStartupProbeFailureRegExpStr = `ns/(?P<NS>openshift-marketplace) pod/(?P<POD>(community-operators|redhat-operators)-[a-z0-9-]+).*Startup probe failed`
)

func CombinedRegexp(arr ...*regexp.Regexp) *regexp.Regexp {
	s := ""
	for _, r := range arr {
		if s != "" {
			s += "|"
		}
		s += r.String()
	}
	return regexp.MustCompile(s)
}

var AllowedRepeatedEventPatterns = []*regexp.Regexp{
	// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should not deadlock when a pod's predecessor fails [Suite:openshift/conformance/parallel] [Suite:k8s]
	// PauseNewPods intentionally causes readiness probe to fail.
	regexp.MustCompile(`ns/e2e-statefulset-[0-9]+ pod/ss-[0-9] node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),

	// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should perform rolling updates and roll backs of template modifications [Conformance] [Suite:openshift/conformance/parallel/minimal] [Suite:k8s]
	// breakPodHTTPProbe intentionally causes readiness probe to fail.
	regexp.MustCompile(`ns/e2e-statefulset-[0-9]+ pod/ss2-[0-9] node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: HTTP probe failed with statuscode: 404`),

	// [sig-node] Probing container ***
	// these tests intentionally cause repeated probe failures to ensure good handling
	regexp.MustCompile(`ns/e2e-container-probe-[0-9]+ .* probe failed: `),
	regexp.MustCompile(`ns/e2e-container-probe-[0-9]+ .* probe warning: `),

	// Kubectl Port forwarding ***
	// The same pod name is used many times for all these tests with a tight readiness check to make the tests fast.
	// This results in hundreds of events while the pod isn't ready.
	regexp.MustCompile(`ns/e2e-port-forwarding-[0-9]+ pod/pfpod node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed:`),

	// should not start app containers if init containers fail on a RestartAlways pod
	// the init container intentionally fails to start
	regexp.MustCompile(`ns/e2e-init-container-[0-9]+ pod/pod-init-[a-z0-9.-]+ node/[a-z0-9.-]+ - reason/BackOff Back-off restarting failed container`),

	// TestAllowedSCCViaRBAC and TestPodUpdateSCCEnforcement
	// The pod is shaped to intentionally not be scheduled.  Looks like an artifact of the old integration testing.
	regexp.MustCompile(`ns/e2e-test-scc-[a-z0-9]+ pod/.* - reason/FailedScheduling.*`),

	// Security Context ** should not run with an explicit root user ID
	// Security Context ** should not run without a specified user ID
	// This container should never run
	regexp.MustCompile(`ns/e2e-security-context-test-[0-9]+ pod/.*-root-uid node/[a-z0-9.-]+ - reason/Failed Error: container's runAsUser breaks non-root policy.*"`),

	// PersistentVolumes-local tests should not run the pod when there is a volume node
	// affinity and node selector conflicts.
	regexp.MustCompile(`ns/e2e-persistent-local-volumes-test-[0-9]+ pod/pod-[a-z0-9.-]+ reason/FailedScheduling`),

	// various DeploymentConfig tests trigger this by canceling multiple rollouts
	regexp.MustCompile(`reason/DeploymentAwaitingCancellation Deployment of version [0-9]+ awaiting cancellation of older running deployments`),

	// this image is used specifically to be one that cannot be pulled in our tests
	regexp.MustCompile(`.*reason/BackOff Back-off pulling image "webserver:404"`),

	// If image pulls in e2e namespaces fail catastrophically we'd expect them to lead to test failures
	// We are deliberately not ignoring image pull failures for core component namespaces
	regexp.MustCompile(`ns/e2e-.* reason/BackOff Back-off pulling image`),

	// promtail crashlooping as its being started by sideloading manifests.  per @vrutkovs
	regexp.MustCompile("ns/openshift-e2e-loki pod/loki-promtail.*Readiness probe"),

	// Related to known bug below, but we do not need to report on loki: https://bugzilla.redhat.com/show_bug.cgi?id=1986370
	regexp.MustCompile("ns/openshift-e2e-loki pod/loki-promtail.*reason/NetworkNotReady"),

	// kube-apiserver guard probe failing due to kube-apiserver operands getting rolled out
	// multiple times during the bootstrapping phase of a cluster installation
	regexp.MustCompile("ns/openshift-kube-apiserver pod/kube-apiserver-guard.*ProbeError Readiness probe error"),
	// the same thing happens for kube-controller-manager and kube-scheduler
	regexp.MustCompile("ns/openshift-kube-controller-manager pod/kube-controller-manager-guard.*ProbeError Readiness probe error"),
	regexp.MustCompile("ns/openshift-kube-scheduler pod/kube-scheduler-guard.*ProbeError Readiness probe error"),

	// this is the less specific even sent by the kubelet when a probe was executed successfully but returned false
	// we ignore this event because openshift has a patch in patch_prober that sends a more specific event about
	// readiness failures in openshift-* namespaces.  We will catch the more specific ProbeError events.
	regexp.MustCompile("Unhealthy Readiness probe failed"),
	// readiness probe errors during pod termination are expected, so we do not fail on them.
	regexp.MustCompile("TerminatingPodProbeError"),

	// we have a separate test for this
	regexp.MustCompile(OvnReadinessRegExpStr),

	// Separated out in testBackoffPullingRegistryRedhatImage
	regexp.MustCompile(ImagePullRedhatRegEx),

	// Separated out in testRequiredInstallerResourcesMissing
	regexp.MustCompile(RequiredResourcesMissingRegEx),

	// Separated out in testBackoffStartingFailedContainer
	regexp.MustCompile(BackoffRestartingFailedRegEx),

	// Separated out in testErrorUpdatingEndpointSlices
	regexp.MustCompile(ErrorUpdatingEndpointSlicesRegex),

	// If you see this error, it means enough was working to get this event which implies enough retries happened to allow initial openshift
	// installation to succeed. Hence, we can ignore it.
	regexp.MustCompile(`reason/FailedCreate .* error creating EC2 instance: InsufficientInstanceCapacity: We currently do not have sufficient .* capacity in the Availability Zone you requested`),

	// Separated out in testNodeHasNoDiskPressure
	regexp.MustCompile(NodeHasNoDiskPressureRegExpStr),

	// Separated out in testNodeHasSufficientMemory
	regexp.MustCompile(NodeHasSufficientMemoryRegExpStr),

	// Separated out in testNodeHasSufficientPID
	regexp.MustCompile(NodeHasSufficientPIDRegExpStr),

	// Separated out in testMarketplaceStartupProbeFailure
	regexp.MustCompile(MarketplaceStartupProbeFailureRegExpStr),
}

// AllowedUpgradeRepeatedEventPatterns are patterns of events that we should only allow during upgrades, not during normal execution.
var AllowedUpgradeRepeatedEventPatterns = []*regexp.Regexp{
	// Operators that use library-go can report about multiple versions during upgrades.
	regexp.MustCompile(`ns/openshift-etcd-operator deployment/etcd-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-apiserver-operator deployment/kube-apiserver-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-controller-manager-operator deployment/kube-controller-manager-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-scheduler-operator deployment/openshift-kube-scheduler-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),

	// etcd-quorum-guard can fail during upgrades.
	regexp.MustCompile(`ns/openshift-etcd pod/etcd-quorum-guard-[a-z0-9-]+ node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),
	// etcd can have unhealthy members during an upgrade
	regexp.MustCompile(`ns/openshift-etcd-operator deployment/etcd-operator - reason/UnhealthyEtcdMember unhealthy members: .*`),
	// etcd-operator began to version etcd-endpoints configmap in 4.10 as part of static-pod-resource. During upgrade existing revisions will not contain the resource.
	// The condition reconciles with the next revision which the result of the upgrade. TODO(hexfusion) remove in 4.11
	regexp.MustCompile(`ns/openshift-etcd-operator deployment/etcd-operator - reason/RequiredInstallerResourcesMissing configmaps: etcd-endpoints-[0-9]+`),
	// There is a separate test to catch this specific case
	regexp.MustCompile(RequiredResourcesMissingRegEx),

	// Separated out in testMarketplaceStartupProbeFailure
	regexp.MustCompile(MarketplaceStartupProbeFailureRegExpStr),
}
