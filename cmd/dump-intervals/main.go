package main

import (
	"os"

	"github.com/openshift/origin/pkg/monitor/dumpintervals"
)

func main() {
	cmd := dumpintervals.NewDumpIntervalsCommand()

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
