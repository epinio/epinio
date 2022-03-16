package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps Endpoint", func() {
	var (
		namespace string
	)
	containerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	It("lists all applications belonging to the namespace", func() {
		app1 := catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)
		defer env.DeleteApp(app1)
		app2 := catalog.NewAppName()
		env.MakeContainerImageApp(app2, 1, containerImageURL)
		defer env.DeleteApp(app2)

		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications",
			serverURL, v1.Root, namespace), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var apps models.AppList
		err = json.Unmarshal(bodyBytes, &apps)
		Expect(err).ToNot(HaveOccurred())

		appNames := []string{apps[0].Meta.Name, apps[1].Meta.Name}
		Expect(appNames).To(ContainElements(app1, app2))

		namespaceNames := []string{apps[0].Meta.Namespace, apps[1].Meta.Namespace}
		Expect(namespaceNames).To(ContainElements(namespace, namespace))

		// Applications are deployed. Must have workload.
		statuses := []string{apps[0].Workload.Status, apps[1].Workload.Status}
		Expect(statuses).To(ContainElements("1/1", "1/1"))
	})

	It("returns a 404 when the namespace does not exist", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/idontexist/applications",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})
})
