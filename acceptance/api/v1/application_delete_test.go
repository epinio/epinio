package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppDelete Endpoint", func() {
	var (
		namespace string
	)
	containerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		// Wait for server to be up and running
		Eventually(func() error {
			_, err := env.Curl("GET", serverURL+v1.Root+"/info", strings.NewReader(""))
			return err
		}, "1m").ShouldNot(HaveOccurred())
	})
	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	It("removes the application, unbinds bound services", func() {
		app1 := catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)
		service := catalog.NewServiceName()
		env.MakeService(service)
		env.BindAppService(app1, service, namespace)
		defer env.CleanupService(service)

		response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/namespaces/%s/applications/%s",
			serverURL, v1.Root, namespace, app1), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		Expect(response.StatusCode).To(Equal(http.StatusOK))
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())

		var resp map[string][]string
		err = json.Unmarshal(bodyBytes, &resp)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp).To(HaveLen(1))
		Expect(resp).To(HaveKey("unboundservices"))
		Expect(resp["unboundservices"]).To(ContainElement(service))
	})

	It("returns a 404 when the namespace does not exist", func() {
		app1 := catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)
		defer env.DeleteApp(app1)

		response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/namespaces/idontexist/applications/%s",
			serverURL, v1.Root, app1), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})

	It("returns a 404 when the app does not exist", func() {
		response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/namespaces/%s/applications/bogus",
			serverURL, v1.Root, namespace), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})
})
