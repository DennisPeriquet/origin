package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/intervalcreation"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func Test_recordAddOrUpdateEvent(t *testing.T) {

	type KubeEventsItems struct {
		Items []corev1.Event `json:"items"`
	}

	// Get the kubeconfig by creating a file using the output of the KAAS tool or cluster-bot.
	// You can possible get panics due to timeout of KAAS kube configs.
	// Get the json file for all events from artifacts/gather-extra.  Or by using using
	// "oc -n aNamespace get event" after setting KUBECONFIG.
	prefix := "/tmp/g/"
	kubeconfig := prefix + "kk.txt"
	//jsonFile := prefix + "e2e-raw-events_20230211-234001.json"  // raw corev1.Events file
	jsonFile := prefix + "e2e-raw-events_20230211-235640.json" // raw corev1.Events file
	artifactDir := prefix + "junit"
	jsonOutFile := prefix + "out.json" // resultant events.json

	file, err := os.Open(jsonFile)
	if err != nil {
		fmt.Println("Error opening jsonFile:", err)
		return
	}
	defer file.Close()

	var kubeEvents KubeEventsItems
	if err := json.NewDecoder(file).Decode(&kubeEvents); err != nil {
		fmt.Println("Error reading jsonFile:", err)
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Println("Unable to setup *rest.Config:", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println("Unable to setup kube client:", err)
	}

	smallKubeEvents := KubeEventsItems{
		Items: []corev1.Event{
			{
				Count:   2,
				Reason:  "NodeHasNoDiskPressure",
				Message: "sample message",
			},
		},
	}
	type args struct {
		ctx                    context.Context
		m                      *Monitor
		client                 kubernetes.Interface
		reMatchFirstQuote      *regexp.Regexp
		significantlyBeforeNow time.Time
		kubeEventList          KubeEventsItems
	}

	tests := []struct {
		name    string
		args    args
		skip    bool
		writeIt bool
	}{
		{
			name: "Single Event test",
			skip: true,
			args: args{
				ctx:                    context.TODO(),
				m:                      NewMonitorWithInterval(time.Second),
				client:                 nil,
				reMatchFirstQuote:      regexp.MustCompile(`"([^"]+)"( in (\d+(\.\d+)?(s|ms)$))?`),
				significantlyBeforeNow: time.Now().UTC().Add(-15 * time.Minute),
				kubeEventList:          smallKubeEvents,
			},
		},
		{
			name:    "Multiple Event (from file) test",
			skip:    false, // skip in case we don't have a file
			writeIt: true,
			args: args{
				ctx:               context.TODO(),
				m:                 NewMonitorWithInterval(time.Second),
				client:            client,
				reMatchFirstQuote: regexp.MustCompile(`"([^"]+)"( in (\d+(\.\d+)?(s|ms)$))?`),

				// Use the timestamp of the first corev1.Event
				significantlyBeforeNow: kubeEvents.Items[0].LastTimestamp.UTC().Add(-15 * time.Minute),
				kubeEventList:          kubeEvents,
			},
		},
	}
	for _, tt := range tests {
		if tt.skip {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			for _, event := range tt.args.kubeEventList.Items {
				recordAddOrUpdateEvent(tt.args.ctx, tt.args.m, tt.args.client, tt.args.reMatchFirstQuote, tt.args.significantlyBeforeNow, &event)
			}
			if tt.writeIt {
				writeOutJson(tt.args.m, jsonOutFile, artifactDir)
			}
		})
	}
}

func writeOutJson(m *Monitor, jsonOutFile string, artifactDir string) {
	monitorserialization.EventsToFile(jsonOutFile, m.UnsortedEvents)
	eir := intervalcreation.NewSpyglassEventIntervalRenderer("spyglass", intervalcreation.BelongsInSpyglass)
	eir.WriteRunData(artifactDir, m.recordedResources, m.UnsortedEvents, "-0001")
}
