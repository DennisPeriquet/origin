package dumpintervalsoperators

import (
	"fmt"
	"os"
	"time"

	"github.com/openshift/origin/pkg/monitor"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

//func logIt(str string, err error) {
//	fmt.Println(str, err)
//}

//func logFatal(str string, err error) {
//	logIt(str, err)
//	os.Exit(1)
//}

type DumpOperatorsCreateFlags struct {
	// jsonBytes is filled in after marshalling the json taken from jsonFilename
	filename string
}

type DumpOperatorsCreateOptions struct {
	// jsonFilename is the name of the file that holds the events.json produced by pods.go
	jsonFilename string
}

func NewDumpOperatorsCreateFlags() *DumpOperatorsCreateFlags {
	return &DumpOperatorsCreateFlags{}
}

func (f *DumpOperatorsCreateFlags) BindFlags(fs *pflag.FlagSet) {
	fs.StringVar(&f.filename, "json-file", f.filename, "name of operators events json file")
}

func NewDumpOperatorsCommand() *cobra.Command {
	f := NewDumpOperatorsCreateFlags()

	cmd := &cobra.Command{
		Use:          "operators",
		Short:        "Dump the operators html file",
		Long:         `Dump the operators html file (e2e-operators_<date>-<num>.html)`,
		SilenceUsage: false,

		RunE: func(cmd *cobra.Command, args []string) error {

			if err := f.Validate(); err != nil {
				logrus.WithError(err).Fatal("Flags are invalid")
				//fmt.Println("Flags are invalid", err)
			}

			o, err := f.ToOptions()
			if err != nil {
				logrus.WithError(err).Fatal("Failed to build runtime options")
				//fmt.Println("Failed to build runtime options", err)
				os.Exit(1)
			}

			if err := o.Run(); err != nil {
				logrus.WithError(err).Fatal("Command failed")
				//fmt.Println("Command failed", err)
			}

			return nil
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	f.BindFlags(cmd.Flags())

	return cmd
}

// Validate checks to see that all the required arguments are passed.
func (f *DumpOperatorsCreateFlags) Validate() error {

	if len(f.filename) == 0 {
		return fmt.Errorf("--json-file is missing")
	}
	// We can check that the file iname in f.jsonFilename exists and if not return error
	return nil
}

// ToOptions goes from the user input to the runtime values need to run the command.
func (f *DumpOperatorsCreateFlags) ToOptions() (*DumpOperatorsCreateOptions, error) {

	// Confirm the f.filename exists and something we can open
	_, err := os.Stat(f.filename)
	if err != nil {
		return nil, err
	}

	return &DumpOperatorsCreateOptions{
		jsonFilename: f.filename,
	}, nil
}

func (o *DumpOperatorsCreateOptions) Run() error {
	fmt.Println("Filename  = ", o.jsonFilename)

	// https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/origin-ci-test/pr-logs/pull/26892/pull-ci-openshift-origin-master-e2e-aws-serial/1502030011752779776/artifacts/e2e-aws-serial/openshift-e2e-test/artifacts/junit/e2e-events_20220310-224620.json
	// start time March 10, 2022 9:15:44 PM
	// end time  March 11, 2022 12:44:01 AM
	start, err := time.Parse(time.RFC3339, "2022-03-10T21:15:44Z")
	if err != nil {
		fmt.Println("Error parsing start")
		os.Exit(1)
	}
	opt := ginkgo.NewOptions()
	opt.JUnitDir = "/home/dperique/mygit/dperique/NG/Dennis/Redhat/origin/output"

	m := monitor.NewMonitorWithInterval(time.Second)

	fmt.Println("Transforming json file to events (Instants) to use as input ...")
	inputIntervals, err := monitorserialization.EventsFromFile(o.jsonFilename)
	if err != nil {
		logrus.WithError(err).Fatal("Error transforming file to events")
	}

	//sort.Stable(intervalcreation.ByPodLifecycle(inputIntervals))
	m.SetUnsortedEvents(inputIntervals)

	timeSuffix := fmt.Sprintf("_%s", start.UTC().Format("20060102-150405"))
	events := m.Intervals(time.Time{}, time.Time{})

	if len(opt.JUnitDir) > 0 {
		if err := opt.WriteRunDataToArtifactsDir(opt.JUnitDir, m, events, timeSuffix); err != nil {
			fmt.Fprintf(opt.ErrOut, "error: Failed to write run-data: %v\n", err)
		}
	}
	return nil
}
