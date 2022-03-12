package dumpintervals

import (
	"github.com/openshift/origin/pkg/monitor/dumpintervals/dumpintervalseverything"
	"github.com/openshift/origin/pkg/monitor/dumpintervals/dumpintervalsoperators"
	"github.com/spf13/cobra"
)

func NewDumpIntervalsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump-intervals",
		Short: "Dump the itervals html files",
		Long:  `Given a events json file, dump the interval html files`,
	}

	cmd.AddCommand(dumpintervalseverything.NewDumpEverythingCommand())
	cmd.AddCommand(dumpintervalsoperators.NewDumpOperatorsCommand())

	return cmd
}
