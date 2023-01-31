package ginkgo

import (
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

func TestMonitorEventsOptions_WriteRunDataToArtifactsDir(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "Basic test",
			wantErr: false,
		},
	}
	o := NewMonitorEventsOptions(os.Stdout, os.Stdout)

	// We will take this .json file and turn it into one with pathological/true annotation for any event that ends in "(n times)".
	// You can then embed that into an html template.
	//recordedEvents, err := monitorserialization.EventsFromFile("/home/dperique/mygit/dperique/NG/Dennis/Redhat/PR/origin/e2e-events_20230127-133940.json")
	//recordedEvents, err := monitorserialization.EventsFromFile("/home/dperique/mygit/dperique/NG/Dennis/Redhat/PR/origin/e2e-events_20230130-213035.json")

	// interesting picture
	//recordedEvents, err := monitorserialization.EventsFromFile("/home/dperique/mygit/dperique/NG/Dennis/Redhat/PR/origin/e2e-events_20230130-212059.json")

	recordedEvents, err := monitorserialization.EventsFromFile("/home/dperique/mygit/dperique/NG/Dennis/Redhat/PR/origin/e2e-events_20230130-223114.json")

	// no (n times) here
	//recordedEvents, err := monitorserialization.EventsFromFile("/home/dperique/mygit/dperique/NG/Dennis/Redhat/PR/origin/e2e-timelines_openshift-control-plane_20230109-153744.json")
	if err != nil {
		fmt.Println("Events are bad")
	}

	pathologicalMessagePattern := regexp.MustCompile(`(?s)(.*) \((\d+) times\).*`)

	length := len(recordedEvents)
	startTime := recordedEvents[0].From
	endTime := recordedEvents[length-1].To
	o.startTime = &startTime
	o.endTime = &endTime

	var newIntervals monitorapi.Intervals
	for _, e := range recordedEvents {
		if pathologicalMessagePattern.MatchString(e.Message) {
			message := fmt.Sprintf("pathological/true %s", e.Message)
			newIntervals = append(newIntervals, monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   e.Level,
					Locator: e.Locator,
					Message: message,
				},
				From: e.From,
				To:   e.From.Add(time.Second * 1),
			})
		} else {
			newIntervals = append(newIntervals, monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   e.Level,
					Locator: e.Locator,
					Message: e.Message,
				},
				From: e.From,
				To:   e.To,
			})
		}
	}
	o.recordedEvents = newIntervals

	//endDate, _ := time.Parse("2006-01-02-03T04:05:06", "2023-01-27T14:26:25Z")
	// Take from the last event from the sample e2-events json file.
	//startDate := time.Unix(1674825286, 0) // January 27, 2023 1:14:46 PM
	//endDate := time.Unix(1674829585, 0)   // January 27, 2023 2:26:25 PM
	//startDate := time.Unix(1673276717, 0) // January 27, 2023 1:14:46 PM
	//endDate := time.Unix(1673281363, 0)   // January 27, 2023 2:26:25 PM
	//o.startTime = &startDate
	//o.endTime = &endDate
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := o.WriteRunDataToArtifactsDir("/tmp/junit/", "-00001"); (err != nil) != tt.wantErr {
				t.Errorf("MonitorEventsOptions.WriteRunDataToArtifactsDir() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
