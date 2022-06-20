package ginkgo

import (
	"bytes"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// JUnitsForEvents returns a set of JUnit results for the provided events encountered
// during a test suite run.
// Objects that implement JUnitsForEvents, return a list of JUnitTestCases pointers.
type JUnitsForEvents interface {
	// JUnitsForEvents returns a set of additional test passes or failures implied by the
	// events sent during the test suite run. If passed is false, the entire suite is failed.
	// To set a test as flaky, return a passing and failing JUnitTestCase with the same name.
	JUnitsForEvents(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string) []*junitapi.JUnitTestCase
}

// JUnitForEventsFunc converts a function into the JUnitForEvents interface.
// kubeClientConfig may or may not be present.  The JUnit evaluation needs to tolerate a missing *rest.Config
// and an unavailable cluster without crashing.
// DP: this is strange to me
// JUnitForEventsFunc implements the JUnitsForEvents interface.
// StableSystemEventInvariants is of this type and implements the JUnitsForEvents interface.
// SystemUpgradeEventInvariants is of this type and implements the JUnitsForEvents interface.
type JUnitForEventsFunc func(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string) []*junitapi.JUnitTestCase

// JUnitsForEvents for JUnitForEventsFunc calls the function with parameters passed and returns
// the resulting JUnitTestCases.
func (fn JUnitForEventsFunc) JUnitsForEvents(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string) []*junitapi.JUnitTestCase {
	return fn(events, duration, kubeClientConfig, testSuite)
}

// JUnitsForAllEvents aggregates multiple JUnitsForEvent interfaces and returns
// the result of all invocations. It ignores nil interfaces.
// JUnitForAllEvents implements the JUnitsForEvents interface.
type JUnitsForAllEvents []JUnitsForEvents

// JUnitsForEvents goes through all JUnitsForEvents, runs them, and returns a list of the JUnitTestCases.  It
// is called with syntheticEventTests.
// JUnitForAllEvents implements the JUnitsForEvents interface.
func (a JUnitsForAllEvents) JUnitsForEvents(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string) []*junitapi.JUnitTestCase {
	var all []*junitapi.JUnitTestCase
	for _, obj := range a {
		if obj == nil {
			continue
		}
		results := obj.JUnitsForEvents(events, duration, kubeClientConfig, testSuite)
		all = append(all, results...)
	}
	return all
}

// createSyntheticTestsFromMonitor returns a junit test that flakes if there are any EventIntervals
// with level of Error; this is a single test (or two tests if failed/flaked).
func createSyntheticTestsFromMonitor(events monitorapi.Intervals, monitorDuration time.Duration) ([]*junitapi.JUnitTestCase, *bytes.Buffer, *bytes.Buffer) {
	var syntheticTestResults []*junitapi.JUnitTestCase

	buf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	fmt.Fprintf(buf, "\nTimeline:\n\n")

	// Count any EventIntervals that have an error level of Error
	// The output and failure output is stored in the byte buffers to add in
	// the returned junit
	errorCount := 0
	for _, event := range events {
		if event.Level == monitorapi.Error {
			errorCount++
			fmt.Fprintln(errBuf, event.String())
		}
		fmt.Fprintln(buf, event.String())
	}
	fmt.Fprintln(buf)

	// Create a synthetic test that fails if there are any errors but make it a flake.
	monitorTestName := "[sig-arch] Monitor cluster while tests execute"
	if errorCount > 0 {
		syntheticTestResults = append(
			syntheticTestResults,
			&junitapi.JUnitTestCase{
				Name:      monitorTestName,
				SystemOut: buf.String(),
				Duration:  monitorDuration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("%d error level events were detected during this test run:\n\n%s", errorCount, errBuf.String()),
				},
			},
			// write a passing test to trigger detection of this issue as a flake, indicating we have no idea whether
			// these are actual failures or not
			&junitapi.JUnitTestCase{
				Name:     monitorTestName,
				Duration: monitorDuration.Seconds(),
			},
		)
	} else {
		// even if no error events, add a passed test including the output so we can scan with search.ci:
		syntheticTestResults = append(
			syntheticTestResults,
			&junitapi.JUnitTestCase{
				Name:      monitorTestName,
				Duration:  monitorDuration.Seconds(),
				SystemOut: buf.String(),
			},
		)
	}

	return syntheticTestResults, buf, errBuf
}
