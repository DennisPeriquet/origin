package dumpintervalseverything

import (
	"testing"
)

func Test_Everything_Run(t *testing.T) {
	o := DumpEverythingCreateOptions{
		jsonFilename: "/home/dperique/mygit/dperique/NG/Dennis/Redhat/origin/testdata/e2e-intervals_everything_20220301-164208.json",
	}
	err := o.Run()
	if err != nil {
		t.Errorf("Unable to process %s", o.jsonFilename)
	}

	// TODO: capture some output and compare
}
