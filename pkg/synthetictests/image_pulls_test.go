package synthetictests

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

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
			junit_tests := testBackoffPullingRegistryRedhatImage(e)
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

func Test_FiletestBackoffPullingRegistryRedhatImage(t *testing.T) {
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

	// We'll take an existing file containing to create an Intervals object
	// and append an EventInterval we want to test.
	// Obtain this file from a junit subdirectory on a prow job run and realize
	// that the file may already contain EventIntervals that the code will parse.
	// Run like this: go test -v -run Test_FiletestBackoffPullingRegistryRedhatImage
	// run click "debug test" in vscode.
	e2e_file := "/home/dperique/origin_testdata/e2e-events_20220322-185856.json"
	e, err := monitorserialization.EventsFromFile(e2e_file)
	if err != nil {
		t.Errorf("Unable to open e2e_file %s: not found", e2e_file)
	}

	// Append the test data to the end of the Intervals
	//
	for _, tt := range tests {
		new_event := monitorapi.EventInterval{
			Condition: monitorapi.Condition{
				Message: tt.message,
			},
			From: time.Unix(1, 0),
			To:   time.Unix(1, 0),
		}
		e = append(e, new_event)
		t.Run(tt.name, func(t *testing.T) {
			junit_tests := testBackoffPullingRegistryRedhatImage(e)
			switch tt.kind {
			case "pass":
				if len(junit_tests) != 1 {
					t.Errorf("This should've been a single passing test, but got %d tests", len(junit_tests))
				}
				if len(junit_tests[0].SystemOut) != 0 {
					t.Errorf("This should've been a pass, but got %s", junit_tests[0].SystemErr)
				}
			case "fail":
				// At this time, we always want this case to be a flake; so, this will always flake
				// since failureThreshold is maxInt.
				if len(junit_tests) != 2 {
					t.Errorf("This should've been a two tests as flake, but got %d tests", len(junit_tests))
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
