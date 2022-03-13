package dumpintervalsoperators

import (
	"testing"
)

func Test_Operators_Run(t *testing.T) {
	o := DumpOperatorsCreateOptions{
		jsonFilename: "/home/dperique/mygit/dperique/NG/Dennis/Redhat/origin/testdata/operators-e2e-events_20220310-224620.json",
	}
	err := o.Run()
	if err != nil {
		t.Errorf("Unable to process %s", o.jsonFilename)
	}

	// TODO: capture some output and compare
}
