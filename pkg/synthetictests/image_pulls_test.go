package synthetictests

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func Test_testRequiredInstallerResourcesMissing(t *testing.T) {
	tests := []struct {
		name    string
		message string
		kind    string
	}{
		{
			name:    "Test doesn't match but results in passing junit",
			message: "ns/openshift-etcd-operator deployment/etcd-operator - reason/RequiredInstallerYadaMissing secrets: etcd-all-certs-3 (25 times)",
			kind:    "pass",
		},
		{
			name:    "Test failing case",
			message: "ns/openshift-etcd-operator deployment/etcd-operator - reason/RequiredInstallerResourcesMissing secrets: etcd-all-certs-3 (21 times)",
			kind:    "fail",
		},
		{
			name:    "Test flaking case",
			message: "ns/openshift-etcd-operator deployment/etcd-operator - reason/RequiredInstallerResourcesMissing secrets: etcd-all-certs-3 (16 times)",
			kind:    "flake",
		},
		{
			name:    "Test passing case",
			message: "ns/openshift-etcd-operator deployment/etcd-operator - reason/RequiredInstallerResourcesMissing secrets: etcd-all-certs-3 (7 times)",
			kind:    "pass",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Message: tt.message,
					},
					From: time.Unix(1, 0),
					To:   time.Unix(1, 0),
				},
			}
			junit_tests := testRequiredInstallerResourcesMissing(e)
			switch tt.kind {
			case "pass":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single passing test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) != 0 {
					t.Errorf("This should've been a pass, but got %s", junit_tests[0].SystemErr)
				}
			case "fail":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single failing test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) == 0 {
					t.Error("This should've been a failure but got no output")
				}
			case "flake":
				if len(junit_tests) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junit_tests))
				}
			default:
				t.Errorf("Unknown test kind")
			}

		})
	}
}
func Test_testBackoffPullingRegistryRedhatImage(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		num_tests int
		kind      string
	}{
		{
			name:    "Test flake",
			message: `ns/openshift-e2e-loki pod/loki-promtail-ww2rx node/ip-10-0-157-209.us-east-2.compute.internal reason/BackOff Back-off pulling image "registry.redhat.io/openshift4/ose-oauth-proxy:latest" (6 times)`,
			kind:    "flake",
		},
		{
			name:    "Test fail",
			message: `ns/openshift-e2e-loki pod/loki-promtail-ww2rx node/ip-10-0-157-209.us-east-2.compute.internal reason/BackOff Back-off pulling image "registry.redhat.io/openshift4/ose-oauth-proxy:latest" (9 times)`,
			kind:    "fail",
		},
		{
			name:    "Test pass",
			message: `ns/openshift-e2e-loki pod/loki-promtail-qrpkm node/ip-10-0-240-197.us-east-2.compute.internal reason/BackOff Back-off pulling image "registry.not-redhat.io/openshift4/ose-oauth-proxy:latest" (1 times)`,
			kind:    "pass",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Message: tt.message,
					},
					From: time.Unix(1, 0),
					To:   time.Unix(1, 0),
				},
			}
			junitTests := testBackoffPullingRegistryRedhatImage(e)
			switch tt.kind {
			case "pass":
				if len(junitTests) != 1 {
					t.Errorf("This should've been a single passing test, but got %d tests", len(junitTests))
				}
				if len(junitTests[0].SystemOut) != 0 {
					t.Errorf("This should've been a pass, but got %s", junitTests[0].SystemErr)
				}
			case "fail":
				// At this time, we always want this case to be a flake; so, this will always flake
				// since failureThreshold is maxInt.
				if len(junitTests) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junitTests))
				}
			case "flake":
				if len(junitTests) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junitTests))
				}
			default:
				t.Errorf("Unknown test kind")
			}
		})
	}
}

func Test_testBackoffStartingFailedContainer(t *testing.T) {
	tests := []struct {
		name    string
		message string
		kind    string
	}{
		{
			name:    "Test pass case",
			message: "reason/BackOff Back-off restarting failed container (5 times)",
			kind:    "pass",
		},
		{
			name:    "Test failure case",
			message: "reason/BackOff Back-off restarting failed container (56 times)",
			kind:    "fail",
		},
		{
			name:    "Test flake case",
			message: "reason/BackOff Back-off restarting failed container (11 times)",
			kind:    "flake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Message: tt.message,
					},
					From: time.Unix(1, 0),
					To:   time.Unix(1, 0),
				},
			}
			junit_tests := testBackoffStartingFailedContainer(e)
			switch tt.kind {
			case "pass":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single passing test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) != 0 {
					t.Errorf("This should've been a pass, but got %s", junit_tests[0].SystemErr)
				}
			case "fail":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single failing test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) == 0 {
					t.Error("This should've been a failure but got no output")
				}
			case "flake":
				if len(junit_tests) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junit_tests))
				}
			default:
				t.Errorf("Unknown test kind")
			}

		})
	}
}

func Test_testErrorUpdatingEndpointSlices(t *testing.T) {
	tests := []struct {
		name    string
		message string
		kind    string
	}{
		{
			name:    "pass",
			message: "reason/FailedToUpdateEndpointSlices Error updating Endpoint Slices for Service openshift-ovn-kubernetes/ovn-kubernetes-master: node \"ip-10-0-168-211.us-east-2.compute.internal\" not found (2 times)",
			kind:    "pass",
		},
		{
			name:    "fail",
			message: "reason/FailedToUpdateEndpointSlices Error updating Endpoint Slices for Service openshift-ovn-kubernetes/ovn-kubernetes-master: node \"ip-10-0-168-211.us-east-2.compute.internal\" not found (24 times)",
			kind:    "fail",
		},
		{
			name:    "flake",
			message: "reason/FailedToUpdateEndpointSlices Error updating Endpoint Slices for Service openshift-ovn-kubernetes/ovn-kubernetes-master: node \"ip-10-0-168-211.us-east-2.compute.internal\" not found (11 times)",
			kind:    "flake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Message: tt.message,
						Locator: "ns/openshift-ovn-kubernetes service/ovn-kubernetes-master",
					},
					From: time.Unix(1, 0),
					To:   time.Unix(1, 0),
				},
			}
			junit_tests := testErrorUpdatingEndpointSlices(e)
			switch tt.kind {
			case "pass":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single passing test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) != 0 {
					t.Errorf("This should've been a pass, but got %s", junit_tests[0].SystemErr)
				}
			case "fail":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single failing test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) == 0 {
					t.Error("This should've been a failure but got no output")
				}
			case "flake":
				if len(junit_tests) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junit_tests))
				}
			default:
				t.Errorf("Unknown test kind")
			}

		})
	}
}

func Test_testInsufficientInstanceCapacity(t *testing.T) {
	tests := []struct {
		name    string
		message string
		kind    string
	}{
		{
			name: "pass",
			// Get this message from something like http://.../artifacts/e2e-aws-ovn-serial/openshift-e2e-test/artifacts/junit/e2e-events_20220830-004553.json
			message: "reason/FailedCreate ci-op-1: reconciler failed to Create machine: failed to launch instance: error creating EC2 instance: InsufficientInstanceCapacity: We currently do not have sufficient m6a.xlarge capacity in the Availability Zone you requested (us-east-1c). Our system will be working on provisioning additional capacity. You can currently get m6a.xlarge capacity by not specifying an Availability Zone in your request or choosing us-east-1a, us-east-1b, us-east-1d, us-east-1f.\n\tstatus code: 500, request id: c19d038c-b71a-4417-a558-539f021b8916  (2 times)",
			kind:    "pass",
		},
		{
			name:    "fail",
			message: "reason/FailedCreate ci-op-1: reconciler failed to Create machine: failed to launch instance: error creating EC2 instance: InsufficientInstanceCapacity: We currently do not have sufficient m6a.xlarge capacity in the Availability Zone you requested (us-east-1c). Our system will be working on provisioning additional capacity. You can currently get m6a.xlarge capacity by not specifying an Availability Zone in your request or choosing us-east-1a, us-east-1b, us-east-1d, us-east-1f.\n\tstatus code: 500, request id: c19d038c-b71a-4417-a558-539f021b8916 (66 times)",
			kind:    "pass",
		},
		{
			name:    "flake",
			message: "reason/FailedCreate ci-op-1: reconciler failed to Create machine: failed to launch instance: error creating EC2 instance: InsufficientInstanceCapacity: We currently do not have sufficient m6a.xlarge capacity in the Availability Zone you requested (us-east-1c). Our system will be working on provisioning additional capacity. You can currently get m6a.xlarge capacity by not specifying an Availability Zone in your request or choosing us-east-1a, us-east-1b, us-east-1d, us-east-1f.\n\tstatus code: 500, request id: c19d038c-b71a-4417-a558-539f021b8916 (10 times)",
			kind:    "pass",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Message: tt.message,
						// Get this from a place like http://.../artifacts/e2e-aws-ovn-serial/openshift-e2e-test/artifacts/junit/e2e-events_20220830-004553.json
						Locator: "ns/openshift-machine-api machine/ci-op-f1rdy654-0a238-w6v9x-worker-us-east-1c-9shbn",
					},
					From: time.Unix(1, 0),
					To:   time.Unix(1, 0),
				},
			}
			junit_tests := testInsufficientInstanceCapacity(e)
			switch tt.kind {
			case "pass":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single passing test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) != 0 {
					t.Errorf("This should've been a pass, but got %s", junit_tests[0].SystemErr)
				}
			default:
				t.Errorf("Unknown test kind")
			}

		})
	}
}
