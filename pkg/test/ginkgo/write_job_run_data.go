package ginkgo

import (
	"sort"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// RunDataWriter objects write data to the artifacts/junit directory.
type RunDataWriter interface {
	WriteRunData(artifactDir string, monitor *monitor.Monitor, events monitorapi.Intervals, timeSuffix string) error
}

// EventDataWriter objects write data to the artifacts/junit directory.
type EventDataWriter interface {
	WriteEventData(artifactDir string, events monitorapi.Intervals, timeSuffix string) error
}

// RunDataWriterFunc defineds a type that implements the RunDataWriter interface.
type RunDataWriterFunc func(artifactDir string, monitor *monitor.Monitor, events monitorapi.Intervals, timeSuffix string) error

// WriteDataRun writes data to the artifacts/junit directory.
func (fn RunDataWriterFunc) WriteRunData(artifactDir string, monitor *monitor.Monitor, events monitorapi.Intervals, timeSuffix string) error {
	return fn(artifactDir, monitor, events, timeSuffix)
}

// AdaptEventDataWriter returns a RunDataWriterFunc that writes w's events to the artifacts/junit directory.
func AdaptEventDataWriter(w EventDataWriter) RunDataWriterFunc {
	return func(artifactDir string, monitor *monitor.Monitor, events monitorapi.Intervals, timeSuffix string) error {
		return w.WriteEventData(artifactDir, events, timeSuffix)
	}
}

// WriteRunDataToArtifactsDir attempts to write useful run data to the specified directory.
// That specified directory is opt.JUnit directory in artifacts/e2e/artifacts/junit.
// Re: TRT-238, these aritifacts are needed to get Tracked Resources (i.e., pods).
func (opt *Options) WriteRunDataToArtifactsDir(artifactDir string, monitor *monitor.Monitor, unorderedEvents monitorapi.Intervals, timeSuffix string) error {
	errs := []error{}

	// use custom sorting here so that we can prioritize the sort order to make the intervals html page as readable
	// as possible. This makes the events *not* sorted by time -- but sorted by namespace, then time (From, To), then message.
	events := make([]monitorapi.EventInterval, len(unorderedEvents))
	for i := range unorderedEvents {
		events[i] = unorderedEvents[i]
	}
	sort.Stable(monitorapi.ByTimeWithNamespacedPods(events))

	// The opt.RunDataWriters contents (the functions) are assigned in NewOptions.
	for _, writer := range opt.RunDataWriters {

		// Everytime this line runs, something is either written to artifacts/junit or
		// something is computed (i.e., for alerts for TRT-238) using the monitor's
		// contents and then written to artifacts/junit.
		currErr := writer.WriteRunData(artifactDir, monitor, events, timeSuffix)
		if currErr != nil {
			errs = append(errs, currErr)
		}
	}
	return utilerrors.NewAggregate(errs)
}
