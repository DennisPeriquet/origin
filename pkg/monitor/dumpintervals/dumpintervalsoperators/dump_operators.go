package dumpintervalsoperators

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

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
				//logrus.WithError(err).Fatal("Flags are invalid")
				fmt.Println("Flags are invalid", err)
			}

			o, err := f.ToOptions()
			if err != nil {
				//logrus.WithError(err).Fatal("Failed to build runtime options")
				fmt.Println("Failed to build runtime options", err)
				os.Exit(1)
			}

			if err := o.Run(); err != nil {
				//logrus.WithError(err).Fatal("Command failed")
				fmt.Println("Command failed", err)
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
	return nil
}
