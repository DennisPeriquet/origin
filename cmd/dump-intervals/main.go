package main

import (
	"os"

	"github.com/openshift/origin/pkg/monitor/dumpintervals"
	//"github.com/openshift/origin/pkg/test/ginkgo"
)

func main() {
	//opt := ginkgo.NewOptions()
	//fmt.Printf("%v\n", opt)
	cmd := dumpintervals.NewDumpIntervalsCommand()

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
