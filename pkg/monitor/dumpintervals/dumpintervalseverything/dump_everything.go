package dumpintervalseverything

import (
	"fmt"
	"os"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
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

type DumpEverythingCreateFlags struct {
	// filename is passed as an argument on the cli
	filename string
}

type DumpEverythingCreateOptions struct {
	// jsonFilename is the name of the file that holds the events json file and is known
	// to exist
	jsonFilename string
}

func NewDumpEverythingCreateFlags() *DumpEverythingCreateFlags {
	return &DumpEverythingCreateFlags{}
}

func (f *DumpEverythingCreateFlags) BindFlags(fs *pflag.FlagSet) {
	fs.StringVar(&f.filename, "json-file", f.filename, "name of events json file")
}

func NewDumpEverythingCommand() *cobra.Command {
	f := NewDumpEverythingCreateFlags()

	cmd := &cobra.Command{
		Use:          "everything",
		Short:        "Dump the everything html file",
		Long:         `Dump the everything html file (e2e-intervals_everything_<date>-<num>.html)`,
		SilenceUsage: false,

		RunE: func(cmd *cobra.Command, args []string) error {

			if err := f.Validate(); err != nil {
				logrus.WithError(err).Fatal("Flags are invalid")
				//logIt("Flags are invalid", err)
			}

			o, err := f.ToOptions()
			if err != nil {
				logrus.WithError(err).Fatal("Failed to build runtime options")
				//logFatal("Failed to build runtime options", err)
			}

			if err := o.Run(); err != nil {
				logrus.WithError(err).Fatal("Command failed")
				//logIt("Command failed", err)
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
func (f *DumpEverythingCreateFlags) Validate() error {

	if len(f.filename) == 0 {
		return fmt.Errorf("--json-file is missing")
	}
	return nil
}

// ToOptions goes from the user input to the runtime values need to run the command.
func (f *DumpEverythingCreateFlags) ToOptions() (*DumpEverythingCreateOptions, error) {

	// Confirm the f.filename exists and something we can open
	_, err := os.Stat(f.filename)
	if err != nil {
		return nil, err
	}

	return &DumpEverythingCreateOptions{
		jsonFilename: f.filename,
	}, nil
}

func (o *DumpEverythingCreateOptions) Run() error {
	fmt.Println("Reading file: ", o.jsonFilename)
	fmt.Println("Transforming json file to events (Instants) to use as input ...")

	inputIntervals, err := monitorserialization.EventsFromFile(o.jsonFilename)
	fmt.Print(inputIntervals)
	if err != nil {
		logrus.WithError(err).Fatal("Error transforming file to events")
	}

	//sort.Stable(intervalcreation.ByPodLifecycle(inputIntervals))

	// We use this like to test:
	// https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/origin-ci-test/logs/periodic-ci-openshift-release-master-ci-4.10-upgrade-from-stable-4.9-e2e-ovirt-upgrade/1498692901662625792/artifacts/e2e-ovirt-upgrade/openshift-e2e-test/artifacts/junit/e2e-intervals_everything_20220301-164208.json
	//
	// starttime: March 1, 2022 16:13:29
	// endtime  : March 1, 2022 17:52:16

	startTime, err := time.Parse(time.RFC3339, "2022-03-01T16:13:29Z")
	fmt.Print(startTime)
	//startTime, err := time.Parse(time.RFC3339, "2022-03-01T16:42:08Z")
	if err != nil {
		logrus.WithError(err).Fatal("Error setting up start time")
	}

	endTime, err := time.Parse(time.RFC3339, "2022-03-01T17:52:16Z")
	//endTime, err := time.Parse(time.RFC3339, "2022-03-01T17:46:44Z")
	if err != nil {
		logrus.WithError(err).Fatal("Error setting up end time")
	}

	fmt.Println("Creating PodIntervals from Instants ...")
	//result := intervalcreation.CreatePodIntervalsFromInstants(inputIntervals, startTime, endTime)
	result := monitorapi.Intervals{}
	fmt.Print(endTime)

	resultBytes, err := monitorserialization.EventsToJSON(result)
	if err != nil {
		logrus.WithError(err).Fatal("Error translating back to json")
	}
	fmt.Println(string(resultBytes))

	return nil
}
