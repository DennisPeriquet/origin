package operators

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/kube-openapi/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// checkSubresourceStatus returns a list of names of CRDs that have a "status" in the CRD schema
// but no subresource.status defined.
// For now, it ignores the ones that are currently failing.
func checkSubresourceStatus(crdItemList []apiextensionsv1.CustomResourceDefinition) []string {

	// These CRDs, at the time this test was written, do not have a "status" in the CRD schema
	// and subresource.status.
	// These can be skipped for now but we don't want the number to increase.
	// These CRDs should be tidied up over time.
	//
	exceptionsList := sets.NewString(
		"networks.config.openshift.io",
		"networks.operator.openshift.io",
		"operatorpkis.network.operator.openshift.io",
		"profiles.tuned.openshift.io",
		"tuneds.tuned.openshift.io",
	)

	failures := []string{}
	for _, crdItem := range crdItemList {

		// This test is interested only in CRDs that end with "openshift.io".
		if !strings.HasSuffix(crdItem.ObjectMeta.Name, "openshift.io") {
			continue
		}

		crdName := crdItem.ObjectMeta.Name

		// Skip CRDs in the exceptions list for now.
		if exceptionsList.Has(crdName) {
			continue
		}

		// Iterate through all versions of the CustomResourceDefinition Spec looking for one with
		// a schema status element,
		foundStatusInSchema := false
		var i int
		for i = 0; i < len(crdItem.Spec.Versions); i++ {
			if _, ok := crdItem.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["status"]; ok {
				foundStatusInSchema = true
				break
			}
		}

		if foundStatusInSchema {
			if !(crdItem.Spec.Versions[i].Subresources != nil && crdItem.Spec.Versions[i].Subresources.Status != nil) {
				failures = append(failures, fmt.Sprintf("CRD %s has a status in the schema but no subresource.status", crdName))
			}
		}
	}

	return failures
}

// checkStatusInSchema returns a list of names of CRDs that don't have a "status" in the CRD schema.
// For now, it ignores the ones that are currently failing.
func checkStatusInSchema(crdItemList []apiextensionsv1.CustomResourceDefinition) []string {

	// These CRDs, at the time this test was written, do not have a "status" in the CRD schema
	// and subresource.status.
	// These can be skipped for now but we don't want the number to increase.
	// These CRDs should be tidied up over time.
	//
	exceptionsList := sets.NewString(
		"builds.config.openshift.io",
		"clusternetworks.network.openshift.io",
		"consoleclidownloads.console.openshift.io",
		"consoleexternalloglinks.console.openshift.io",
		"consolelinks.console.openshift.io",
		"consolenotifications.console.openshift.io",
		"consoleplugins.console.openshift.io",
		"consolequickstarts.console.openshift.io",
		"consoleyamlsamples.console.openshift.io",
		"egressnetworkpolicies.network.openshift.io",
		"hostsubnets.network.openshift.io",
		"imagecontentpolicies.config.openshift.io",
		"imagecontentsourcepolicies.operator.openshift.io",
		"machineconfigs.machineconfiguration.openshift.io",
		"netnamespaces.network.openshift.io",
		"rangeallocations.security.internal.openshift.io",
		"rolebindingrestrictions.authorization.openshift.io",
		"securitycontextconstraints.security.openshift.io",
	)

	failures := []string{}
	for _, crdItem := range crdItemList {

		// This test is interested only in CRDs that end with "openshift.io".
		if !strings.HasSuffix(crdItem.ObjectMeta.Name, "openshift.io") {
			continue
		}

		crdName := crdItem.ObjectMeta.Name

		// Skip CRDs in the exceptions list for now.
		if exceptionsList.Has(crdName) {
			continue
		}

		// Iterate through all versions of the CustomResourceDefinition Spec looking for one with
		// a schema status element,
		foundStatusInSchema := false
		var i int
		for i = 0; i < len(crdItem.Spec.Versions); i++ {
			if _, ok := crdItem.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["status"]; ok {
				foundStatusInSchema = true
				break
			}
		}

		if !foundStatusInSchema {
			failures = append(failures, fmt.Sprintf("CRD %s has no 'status' element in its schema", crdName))
		}
	}

	return failures
}

// checkStatusInSchema returns a list of names of CRDs that don't have a "status" in the CRD schema.
// For now, it ignores the ones that are currently failing.
func checkFunc(checkObj checkerInt, crdItemList []apiextensionsv1.CustomResourceDefinition) []string {

	// These CRDs, at the time this test was written, do not have a "status" in the CRD schema
	// and subresource.status.
	// These can be skipped for now but we don't want the number to increase.
	// These CRDs should be tidied up over time.
	//
	exceptionsList := checkObj.getExceptions()

	failures := []string{}
	for _, crdItem := range crdItemList {

		// This test is interested only in CRDs that end with "openshift.io".
		if !strings.HasSuffix(crdItem.ObjectMeta.Name, "openshift.io") {
			continue
		}

		crdName := crdItem.ObjectMeta.Name

		// Skip CRDs in the exceptions list for now.
		if exceptionsList.Has(crdName) {
			continue
		}

		// Iterate through all versions of the CustomResourceDefinition Spec looking for one with
		// a schema status element,
		foundStatusInSchema := false
		var i int
		for i = 0; i < len(crdItem.Spec.Versions); i++ {
			if _, ok := crdItem.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["status"]; ok {
				foundStatusInSchema = true
				break
			}
		}

		failed, failureText := checkObj.processFoundState(foundStatusInSchema, crdItem, i)
		if failed {
			failures = append(failures, failureText)
		}
	}

	return failures
}

type checkSubresourceStatusType struct{}
type checkStatusInSchemaType struct{}

func (c checkSubresourceStatusType) processFoundState(foundStatusInSchema bool, crdItem apiextensionsv1.CustomResourceDefinition, versionIndex int) (bool, string) {
	if !foundStatusInSchema {
		return true, fmt.Sprintf("CRD %s has no 'status' element in its schema", crdItem.ObjectMeta.Name)
	}
	return false, ""
}

func (c checkSubresourceStatusType) getExceptions() sets.String {
	return sets.NewString(
		"networks.config.openshift.io",
		"networks.operator.openshift.io",
		"operatorpkis.network.operator.openshift.io",
		"profiles.tuned.openshift.io",
		"tuneds.tuned.openshift.io")
}

func (c checkStatusInSchemaType) processFoundState(foundStatusInSchema bool, crdItem apiextensionsv1.CustomResourceDefinition, versionIndex int) (bool, string) {
	if foundStatusInSchema {
		if !(crdItem.Spec.Versions[versionIndex].Subresources != nil && crdItem.Spec.Versions[versionIndex].Subresources.Status != nil) {
			return true, fmt.Sprintf("CRD %s has a status in the schema but no subresource.status", crdItem.ObjectMeta.Name)
		}
	}
	return false, ""
}

func (c checkStatusInSchemaType) getExceptions() sets.String {
	return sets.NewString(
		"builds.config.openshift.io",
		"clusternetworks.network.openshift.io",
		"consoleclidownloads.console.openshift.io",
		"consoleexternalloglinks.console.openshift.io",
		"consolelinks.console.openshift.io",
		"consolenotifications.console.openshift.io",
		"consoleplugins.console.openshift.io",
		"consolequickstarts.console.openshift.io",
		"consoleyamlsamples.console.openshift.io",
		"egressnetworkpolicies.network.openshift.io",
		"hostsubnets.network.openshift.io",
		"imagecontentpolicies.config.openshift.io",
		"imagecontentsourcepolicies.operator.openshift.io",
		"machineconfigs.machineconfiguration.openshift.io",
		"netnamespaces.network.openshift.io",
		"rangeallocations.security.internal.openshift.io",
		"rolebindingrestrictions.authorization.openshift.io",
		"securitycontextconstraints.security.openshift.io")
}

type checkerType struct {
	checkerInt
}
type checkerInt interface {
	getExceptions() sets.String
	processFoundState(foundStatusInSchema bool, crdItem apiextensionsv1.CustomResourceDefinition, versionIndex int) (bool, string)
}

var _ = g.Describe("[sig-arch][Early]", func() {

	defer g.GinkgoRecover()

	var (
		crdItemList []apiextensionsv1.CustomResourceDefinition
	)

	oc := exutil.NewCLI("subresource-status-check")

	g.BeforeEach(func() {
		var err error
		crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
		crdItemList, err = getCRDItemList(*crdClient)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("CRDs for openshift.io", func() {
		g.It("should have subresource.status", func() {
			subresourceStatusChecker := checkSubresourceStatusType{}
			failures := checkFunc(subresourceStatusChecker, crdItemList)
			//checkSubresourceStatus(crdItemList)
			if len(failures) > 0 {
				e2e.Fail(strings.Join(failures, "\n"))
			}
		})
	})
})

var _ = g.Describe("[sig-arch][Early]", func() {

	defer g.GinkgoRecover()

	var (
		crdItemList []apiextensionsv1.CustomResourceDefinition
	)

	oc := exutil.NewCLI("schema-status-check")

	g.BeforeEach(func() {
		var err error
		crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
		crdItemList, err = getCRDItemList(*crdClient)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("CRDs for openshift.io", func() {
		g.It("should have a status in the CRD schema", func() {

			statusInSchemaChecker := checkStatusInSchemaType{}
			failures := checkFunc(statusInSchemaChecker, crdItemList)
			//failures := checkStatusInSchema(crdItemList)
			if len(failures) > 0 {
				e2e.Fail(strings.Join(failures, "\n"))
			}
		})
	})
})

func getCRDItemList(crdClient apiextensionsclientset.Clientset) ([]apiextensionsv1.CustomResourceDefinition, error) {

	crdList, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return crdList.Items, err
}
