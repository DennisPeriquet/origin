package ginkgo

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/duplicateevents"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

func Test_markMissedPathologicalEvents(t *testing.T) {
	type args struct {
		events monitorapi.Intervals
	}

	// https://prow.ci.openshift.org/view/gs/origin-ci-test/pr-logs/pull/27743/pull-ci-openshift-origin-master-e2e-azure-ovn-etcd-scaling/1628465728585732096
	// https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/origin-ci-test/pr-logs/pull/27743/pull-ci-openshift-origin-master-e2e-azure-ovn-etcd-scaling/1628465728585732096/artifacts/e2e-azure-ovn-etcd-scaling/openshift-e2e-test/artifacts/junit/
	eventListFile := "/tmp/test/e2e-events_20230222-195949.json"
	eventIntervalList, err := monitorserialization.EventsFromFile(eventListFile)
	skipFullTest := false
	if err != nil {
		// If the full file test is not present, we skip the full test.
		skipFullTest = true
	}
	from, err := time.Parse(time.RFC3339, "2023-02-22T20:12:14Z")
	to := from.Add(1 * time.Second)

	if err != nil {
		t.Errorf("Unable to process events file: %s ", eventListFile)
	}
	tests := []struct {
		name string
		skip bool
		size int
		args args
	}{
		{
			name: "full test",
			size: 397,
			skip: skipFullTest,
			args: args{
				events: eventIntervalList,
			},
		},
		{
			name: "two pathos, one previous events each",
			size: 4,
			args: args{
				events: []monitorapi.EventInterval{
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/c6151e47e4",
							Message: "pathological/true reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (153 times)",
						},
						From: from,
						To:   to,
					},
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (8 times)",
						},
						From: from.Add(-90 * time.Second),
						To:   to.Add(-89 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/f33a7e39ac",
							Message: "pathological/true reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (21 times)",
						},
						From: from.Add(-100 * time.Second),
						To:   to.Add(-95 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (19 times)",
						},
						From: from.Add(-110 * time.Second),
						To:   to.Add(-105 * time.Second),
					},
				},
			},
		},
		{
			name: "locatorMatch, msgDifferent",
			size: 1,
			args: args{
				events: []monitorapi.EventInterval{
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/c6151e47e4",
							Message: "pathological/true reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (153 times)",
						},
						From: from,
						To:   to,
					},
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/NetworkNotReady network is wacked: container runtime network is DIFFERENT. Has your DIFFERENT network provider started? (8 times)",
						},
						From: from.Add(-90 * time.Second),
						To:   to.Add(-89 * time.Second),
					},
				},
			},
		},
		{
			name: "locatorDifferent, msgMatch",
			size: 1,
			args: args{
				events: []monitorapi.EventInterval{
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/f33a7e39ac",
							Message: "pathological/true reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (21 times)",
						},
						From: from.Add(-100 * time.Second),
						To:   to.Add(-95 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-DIFFERENT",
							Message: "reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (19 times)",
						},
						From: from.Add(-110 * time.Second),
						To:   to.Add(-105 * time.Second),
					},
				},
			},
		},
		{
			name: "no patho events",
			size: 0,
			args: args{
				events: []monitorapi.EventInterval{
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (5 times)",
						},
						From: from.Add(-100 * time.Second),
						To:   to.Add(-95 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-1",
							Message: "reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-1\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (4 times)",
						},
						From: from.Add(-110 * time.Second),
						To:   to.Add(-105 * time.Second),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		if tt.skip {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			markMissedPathologicalEvents(tt.args.events)

			// All events should have been mutated to contain the pathological/true mark
			count := tt.size
			ll := 0
			for _, event := range tt.args.events {
				if strings.Contains(event.Message, duplicateevents.PathologicalMark) {
					count--
					ll++
				}
			}
			fmt.Println(ll)
			if count != 0 {
				t.Errorf("Events missing pathological/true: %d ", count)
			}
		})
	}
}
