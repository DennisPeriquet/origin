package synthetictests

import (
	"reflect"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

// See Deep's PR: https://github.com/openshift/origin/pull/27826
// I filled this out a little to help get past dealing with the syntax
// of a multi-faceted composite literal.
func Test_dnsOverlapDisruption(t *testing.T) {
	now := time.Now()

	type args struct {
		events monitorapi.Intervals
	}
	tests := []struct {
		name string
		args args
		want []*junitapi.JUnitTestCase
	}{
		{
			name: "case where the call returns a flaking test",
			args : args{
				events: []monitorapi.EventInterval{
					{
						Condition: monitorapi.Condition{
							Locator: "locator",
							Message: "Message",
						},
						From: now.Add(-31 * time.Second),
						To: now.Add(-30 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "locator",
							Message: "Message",
						},
						From: now.Add(-29 * time.Second),
						To: now.Add(-28 * time.Second),
					},
				},
			},
			want: []*junitapi.JUnitTestCase{
				&junitapi.JUnitTestCase{
					Name: "The name of the test",

					// Nothing else goes here to represent a passing test
				},
				&junitapi.JUnitTestCase{
					Name: "The name of the test",

					// Add failure output for a failed test
					FailureOutput: &junitapi.FailureOutput{
						Message: "This is the failure message",
					},
				},
			},
		},
		{
			name: "case where the call returns a passing test",
			args : args{
				events: []monitorapi.EventInterval{
					{
						Condition: monitorapi.Condition{
							Locator: "locator",
							Message: "Message",
						},
						From: now.Add(-31 * time.Second),
						To: now.Add(-30 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "locator",
							Message: "Message",
						},
						From: now.Add(-29 * time.Second),
						To: now.Add(-28 * time.Second),
					},
				},
			},
			want: []*junitapi.JUnitTestCase{
				&junitapi.JUnitTestCase{
					Name: "The name of the test",
				},
			},
		},
		{
			name: "case where the test passes",
			args : args{
				events: []monitorapi.EventInterval{
					{
						Condition: monitorapi.Condition{
							Locator: "locator",
							Message: "Message",
						},
						From: now.Add(-31 * time.Second),
						To: now.Add(-30 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "locator",
							Message: "Message",
						},
						From: now.Add(-29 * time.Second),
						To: now.Add(-28 * time.Second),
					},
				},
			},
			want: []*junitapi.JUnitTestCase{
				&junitapi.JUnitTestCase{
					FailureOutput: &junitapi.FailureOutput{
						Message: "Failure output goes here",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dnsOverlapDisruption(tt.args.events); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dnsOverlapDisruption() = %v, want %v", got, tt.want)
			}
		})
	}
}
