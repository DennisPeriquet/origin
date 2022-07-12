package synthetictests

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
)

func testAlerts(events monitorapi.Intervals, restConfig *rest.Config, duration time.Duration, m *monitor.Monitor) []*junitapi.JUnitTestCase {
	ret := []*junitapi.JUnitTestCase{}

	alertTests := allowedalerts.AllAlertTests(context.TODO(), restConfig, duration)

	currResourceState := m.CurrentResourceState()
	podResources := currResourceState["pods"]

	for i := range alertTests {
		alertTest := alertTests[i]

		junit, err := alertTest.InvariantCheck(context.TODO(), restConfig, events, podResources)
		if err != nil {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: alertTest.InvariantTestName(),
				FailureOutput: &junitapi.FailureOutput{
					Output: err.Error(),
				},
				SystemOut: err.Error(),
			})
		}
		ret = append(ret, junit...)
	}

	return ret
}
