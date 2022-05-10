package monitor_cmd

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/origin/pkg/monitor"

	"github.com/openshift/origin/pkg/monitor/intervalcreation"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/test/extended/testdata"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type TimelineOptions struct {
	MonitorEventFilename string
	PodResourceFilename  string
	TimelineType         string

	LocatorMatchers        []string
	removedLocatorMatchers []string
	Namespaces             []string
	OutputType             string
	EndDate                string

	KnownRenderers map[string]RenderFunc
	KnownTimelines map[string]monitorapi.EventIntervalMatchesFunc
	IOStreams      genericclioptions.IOStreams
}

type RenderFunc func(intervals monitorapi.Intervals) ([]byte, error)

func NewTimelineOptions(ioStreams genericclioptions.IOStreams) *TimelineOptions {
	return &TimelineOptions{
		TimelineType: "spyglass",

		OutputType: "html",

		IOStreams: ioStreams,
		KnownRenderers: map[string]RenderFunc{
			"json": monitorserialization.EventsToJSON,
			"html": renderHTML,
		},
		KnownTimelines: map[string]monitorapi.EventIntervalMatchesFunc{
			"everything":    intervalcreation.BelongsInEverything,
			"operators":     intervalcreation.BelongsInOperatorRollout,
			"apiserver":     intervalcreation.BelongsInKubeAPIServer,
			"spyglass":      intervalcreation.BelongsInSpyglass,
			"pod-lifecycle": intervalcreation.IsOriginalPodEvent,
		},
	}
}

func NewTimelineCommand(ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := NewTimelineOptions(ioStreams)

	cmd := &cobra.Command{
		Use:   "timeline",
		Short: "Run an upgrade suite",
		Long: `
		Create a timeline html page based on the provided monitor events.

		openshift-tests timeline --type=pod -f raw-monitor-events.json --namespace=openshift-kube-apiserver --namespace=openshift-kube-apiserver-operator -ojson 
		`,

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.ToTimeline().Run(); err != nil {
				return err
			}
			return nil
		},
	}

	o.Bind(cmd.Flags())

	return cmd
}

func (o *TimelineOptions) Bind(flagset *pflag.FlagSet) error {
	flagset.StringVarP(&o.MonitorEventFilename, "filename", "f", o.MonitorEventFilename, "raw-monitor-events.json file")
	flagset.StringSliceVar(&o.Namespaces, "namespace", o.Namespaces, "namespaces to filter.  No entry is no filtering.")
	flagset.StringVarP(&o.OutputType, "output", "o", o.OutputType, fmt.Sprintf("type of output: [%s]", strings.Join(sets.StringKeySet(o.KnownRenderers).List(), ",")))
	flagset.StringVar(&o.TimelineType, "type", o.TimelineType, "type of timeline to produce: "+strings.Join(sets.StringKeySet(o.KnownTimelines).List(), ","))
	flagset.StringVar(&o.PodResourceFilename, "known-pods", o.PodResourceFilename, "resource-pods_<timestamp>.zip filename from openshift-tests.")
	flagset.StringSliceVarP(&o.LocatorMatchers, "locator", "l", o.LocatorMatchers, "key=value selector for monitor event locators (where value is a regex).  for instance -lpod=openshift-etcd-installer.  The same key listed multiple times means an OR.  Each separate key is logically ANDed")
	flagset.StringSliceVarP(&o.removedLocatorMatchers, "remove", "r", o.removedLocatorMatchers, "key=val selector to remove monitor event locators")
	flagset.StringVarP(&o.EndDate, "end-date", "e", o.EndDate, "End date (default is one hour after latest event)")

	return nil
}

func (o *TimelineOptions) Complete() error {
	return nil
}

func (o *TimelineOptions) Validate() error {
	if len(o.MonitorEventFilename) == 0 {
		return fmt.Errorf("missing -f")
	}
	if len(o.OutputType) == 0 {
		return fmt.Errorf("missing -o")
	}
	if len(o.TimelineType) == 0 {
		return fmt.Errorf("missing --type")
	}

	if o.KnownRenderers[o.OutputType] == nil {
		return fmt.Errorf("unknown --type")
	}
	if o.KnownTimelines[o.TimelineType] == nil {
		return fmt.Errorf("unknown --type")
	}

	for _, matcher := range o.LocatorMatchers {
		if !strings.Contains(matcher, "=") {
			return fmt.Errorf("invalid --locator format, must be key=value")
		}
	}

	for _, removedMatcher := range o.removedLocatorMatchers {
		if !strings.Contains(removedMatcher, "=") {
			return fmt.Errorf("invalid --remove format, must be key=value")
		}
	}

	if len(o.EndDate) == 0 {
		// Nothing specified for end-date so make it default.
		o.EndDate = "default"
	}
	if o.EndDate != "default" {
		_, err := time.Parse(time.RFC3339, o.EndDate)
		if err != nil {
			return fmt.Errorf("The --end-date value needs to be a valid time")
		}
	}
	return nil
}

func (o *TimelineOptions) ToTimeline() *Timeline {
	locatorMatcher := map[string][]*regexp.Regexp{}
	inverseLocatorMatcher := map[string][]*regexp.Regexp{}

	for _, matcherString := range o.LocatorMatchers {
		parts := strings.SplitN(matcherString, "=", 2)
		regExp := regexp.MustCompile(parts[1])
		locatorMatcher[parts[0]] = append(locatorMatcher[parts[0]], regExp)
	}

	for _, matcherString := range o.removedLocatorMatchers {
		parts := strings.SplitN(matcherString, "=", 2)
		regExp := regexp.MustCompile(parts[1])
		inverseLocatorMatcher[parts[0]] = append(inverseLocatorMatcher[parts[0]], regExp)
	}

	return &Timeline{
		MonitorEventFilename: o.MonitorEventFilename,
		PodResourceFilename:  o.PodResourceFilename,

		LocatorMatcher:        locatorMatcher,
		RemovedLocatorMatcher: inverseLocatorMatcher,
		Namespaces:            o.Namespaces,
		EndDate:               o.EndDate,

		Renderer:       o.KnownRenderers[o.OutputType],
		TimelineFilter: o.KnownTimelines[o.TimelineType],
		IOStreams:      o.IOStreams,
	}
}

type Timeline struct {
	MonitorEventFilename string
	PodResourceFilename  string

	LocatorMatcher        map[string][]*regexp.Regexp
	RemovedLocatorMatcher map[string][]*regexp.Regexp
	Namespaces            []string
	EndDate               string

	Renderer       RenderFunc
	TimelineFilter monitorapi.EventIntervalMatchesFunc

	IOStreams genericclioptions.IOStreams
}

func (o *Timeline) Run() error {
	consumedEvents, err := monitorserialization.EventsFromFile(o.MonitorEventFilename)
	if err != nil {
		return err
	}

	recordedResources := monitorapi.ResourcesMap{}
	if len(o.PodResourceFilename) > 0 {
		recordedResources, err = loadKnownPods(o.PodResourceFilename)
		if err != nil {
			return err
		}
	}

	filteredEvents := consumedEvents.Filter(o.TimelineFilter)
	if len(o.Namespaces) > 0 {
		filteredEvents = filteredEvents.Filter(monitorapi.IsInNamespaces(sets.NewString(o.Namespaces...)))
	}
	if len(o.LocatorMatcher) > 0 {
		filteredEvents = filteredEvents.Filter(monitorapi.ContainsAllParts(o.LocatorMatcher))
	}

	if len(o.RemovedLocatorMatcher) > 0 {
		filteredEvents = filteredEvents.Filter(monitorapi.NotContainsAllParts(o.RemovedLocatorMatcher))
	}
	// compute intervals from raw
	from := time.Time{}
	var to time.Time

	if o.EndDate == "default" {
		// Limit the final timestamp "To" to one hour after the latest "To" value.
		to = filteredEvents[0].To
		for _, e := range filteredEvents[1:] {
			if to.Before(e.To) {
				to = e.To
			}
		}
		to = to.Add(1 * time.Hour)
	} else {
		to, _ = time.Parse(time.RFC3339, o.EndDate)
	}
	computedIntervalFns := monitor.DefaultIntervalCreationFns()
	for _, createIntervals := range computedIntervalFns {
		filteredEvents = append(filteredEvents, createIntervals(filteredEvents, recordedResources, from, to)...)
	}
	sort.Sort(filteredEvents)

	output, err := o.Renderer(filteredEvents)
	if err != nil {
		return err
	}

	if _, err := o.IOStreams.Out.Write(output); err != nil {
		return err
	}

	return nil
}

func renderHTML(events monitorapi.Intervals) ([]byte, error) {
	eventIntervalsJSON, err := monitorserialization.EventsIntervalsToJSON(events)
	if err != nil {
		return nil, err

	}
	e2eChartTemplate := testdata.MustAsset("e2echart/e2e-chart-template.html")
	e2eChartTitle := "Timeline"
	e2eChartHTML := bytes.ReplaceAll(e2eChartTemplate, []byte("EVENT_INTERVAL_TITLE_GOES_HERE"), []byte(e2eChartTitle))
	e2eChartHTML = bytes.ReplaceAll(e2eChartHTML, []byte("EVENT_INTERVAL_JSON_GOES_HERE"), eventIntervalsJSON)

	return e2eChartHTML, nil
}

func loadKnownPods(filename string) (monitorapi.ResourcesMap, error) {

	zipReader, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer zipReader.Close()

	pods := monitorapi.InstanceMap{}
	for _, f := range zipReader.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		currBytes, err := ioutil.ReadAll(rc)
		if err != nil {
			return nil, err
		}
		_ = rc.Close()

		// there has to be a better way, but this functions, ugly as it is.
		unstructuredObject := map[string]interface{}{}
		if err := json.Unmarshal(currBytes, &unstructuredObject); err != nil {
			return nil, fmt.Errorf("error unmarshalling list: %w", err)
		}
		unstructuredObj := &unstructured.Unstructured{
			Object: unstructuredObject,
		}
		unstructuredList, err := unstructuredObj.ToList()
		if err != nil {
			return nil, err
		}
		//nsList := &unstructured.UnstructuredList{}

		for _, item := range unstructuredList.Items {
			item.SetKind("Pod")
			item.SetAPIVersion("v1")

			pod := &corev1.Pod{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, pod)
			if err != nil {
				return nil, err
			}
			podKey, err := cache.MetaNamespaceKeyFunc(pod)
			if err != nil {
				return nil, err
			}
			pods[podKey] = pod
		}
	}

	return monitorapi.ResourcesMap{
		"pods": pods,
	}, nil
}
