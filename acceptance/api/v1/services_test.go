package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/helpers"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Services API Application Endpoints", func() {
	containerImageURL := "splatform/sample-app"

	var namespace string
	var svc1, svc2 string

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		svc1 = catalog.NewServiceName()
		svc2 = catalog.NewServiceName()

		env.MakeService(svc1)
		env.MakeService(svc2)
	})

	AfterEach(func() {
		env.TargetNamespace(namespace)
		env.DeleteService(svc1)
		env.DeleteService(svc2)
	})

	Describe("GET /api/v1/namespaces/:namespace/services", func() {
		var serviceNames []string

		It("lists all services in the namespace", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/services",
				serverURL, api.Root, namespace), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			var data models.ServiceResponseList
			err = json.Unmarshal(bodyBytes, &data)
			Expect(err).ToNot(HaveOccurred())
			serviceNames = append(serviceNames, data[0].Meta.Name)
			serviceNames = append(serviceNames, data[1].Meta.Name)
			Expect(serviceNames).Should(ContainElements(svc1, svc2))
		})

		It("returns a 404 when the namespace does not exist", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/idontexist/services",
				serverURL, api.Root),
				strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	Describe("GET /api/v1/services", func() {
		var namespace1 string
		var namespace2 string
		var service1 string
		var service2 string
		var app1 string

		// Setting up:
		// namespace1 service1 app1
		// namespace2 service1
		// namespace2 service2

		BeforeEach(func() {
			namespace1 = catalog.NewNamespaceName()
			namespace2 = catalog.NewNamespaceName()
			service1 = catalog.NewServiceName()
			service2 = catalog.NewServiceName()
			app1 = catalog.NewAppName()

			env.SetupAndTargetNamespace(namespace1)
			env.MakeService(service1)
			env.MakeContainerImageApp(app1, 1, containerImageURL)
			env.BindAppService(app1, service1, namespace1)

			env.SetupAndTargetNamespace(namespace2)
			env.MakeService(service1) // separate from namespace1.service1
			env.MakeService(service2)
		})

		AfterEach(func() {
			env.TargetNamespace(namespace2)
			env.DeleteService(service1)
			env.DeleteService(service2)

			env.TargetNamespace(namespace1)
			env.DeleteApp(app1)
			env.DeleteService(service1)
		})

		It("lists all services belonging to all namespaces", func() {
			// But we care only about the three we know about from the setup.

			response, err := env.Curl("GET", fmt.Sprintf("%s%s/services",
				serverURL, api.Root), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			var services models.ServiceResponseList
			err = json.Unmarshal(bodyBytes, &services)
			Expect(err).ToNot(HaveOccurred())

			// `services` contains all services. Not just the two we are looking for, from
			// the setup of this test. Everything which still exists from other tests
			// executing concurrently, or not cleaned by previous tests, or the setup, or
			// ... So, we cannot be sure that the two services are in the two first
			// elements of the slice.

			var serviceRefs [][]string
			for _, s := range services {
				serviceRefs = append(serviceRefs, []string{
					s.Meta.Name,
					s.Meta.Namespace,
					strings.Join(s.Configuration.BoundApps, ", "),
				})
			}
			Expect(serviceRefs).To(ContainElements(
				[]string{service1, namespace1, app1},
				[]string{service1, namespace2, ""},
				[]string{service2, namespace2, ""},
			))

		})
	})

	Describe("GET /api/v1/namespaces/:namespace/services/:service", func() {
		It("lists the service data", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/services/%s",
				serverURL, api.Root, namespace, svc1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			var data models.ServiceResponse
			err = json.Unmarshal(bodyBytes, &data)
			service := data.Configuration.Details
			Expect(err).ToNot(HaveOccurred())
			Expect(service["username"]).To(Equal("epinio-user"))
		})

		It("returns a 404 when the namespace does not exist", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/idontexist/services/%s",
				serverURL, api.Root, svc1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})

		It("returns a 404 when the service does not exist", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/services/bogus",
				serverURL, api.Root, namespace), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	Describe("PATCH /api/v1/namespaces/:namespace/services/:service", func() {
		var changeRequest string
		BeforeEach(func() {
			changeRequest = `{ "remove": ["username"], "edit": { "user" : "ci/cd", "host" : "up" } }`
		})

		It("edits the service", func() {
			// perform the editing

			response, err := env.Curl("PATCH", fmt.Sprintf("%s%s/namespaces/%s/services/%s",
				serverURL, api.Root, namespace, svc1), strings.NewReader(changeRequest))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			var responseData models.Response
			err = json.Unmarshal(bodyBytes, &responseData)
			Expect(err).ToNot(HaveOccurred())

			// then query the service and confirm the changes

			responseGet, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/services/%s",
				serverURL, api.Root, namespace, svc1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(responseGet).ToNot(BeNil())
			defer responseGet.Body.Close()
			Expect(responseGet.StatusCode).To(Equal(http.StatusOK))
			bodyBytesGet, err := ioutil.ReadAll(responseGet.Body)
			Expect(err).ToNot(HaveOccurred())

			var data models.ServiceResponse
			err = json.Unmarshal(bodyBytesGet, &data)
			service := data.Configuration.Details

			Expect(err).ToNot(HaveOccurred())
			Expect(service["user"]).To(Equal("ci/cd"))
			Expect(service["host"]).To(Equal("up"))
			Expect(service).ToNot(HaveKey("username"))
		})

		It("returns a 404 when the namespace does not exist", func() {
			response, err := env.Curl("PATCH", fmt.Sprintf("%s%s/namespaces/idontexist/services/%s",
				serverURL, api.Root, svc1), strings.NewReader(changeRequest))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})

		It("returns a 404 when the service does not exist", func() {
			response, err := env.Curl("PATCH", fmt.Sprintf("%s%s/namespaces/%s/services/bogus",
				serverURL, api.Root, namespace), strings.NewReader(changeRequest))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	Describe("PUT /api/v1/namespaces/:namespace/services/:service", func() {
		var changeRequest string
		BeforeEach(func() {
			changeRequest = `{ "put_key1" : "put_value" }`
		})

		It("replace the service", func() {
			// perform the editing

			response, err := env.Curl("PUT", fmt.Sprintf("%s%s/namespaces/%s/services/%s",
				serverURL, api.Root, namespace, svc1), strings.NewReader(changeRequest))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			var responseData models.Response
			err = json.Unmarshal(bodyBytes, &responseData)
			Expect(err).ToNot(HaveOccurred())

			// then query the service and confirm the changes

			responseGet, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/services/%s",
				serverURL, api.Root, namespace, svc1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(responseGet).ToNot(BeNil())
			defer responseGet.Body.Close()
			Expect(responseGet.StatusCode).To(Equal(http.StatusOK))
			bodyBytesGet, err := ioutil.ReadAll(responseGet.Body)
			Expect(err).ToNot(HaveOccurred())

			var data models.ServiceResponse
			err = json.Unmarshal(bodyBytesGet, &data)
			service := data.Configuration.Details

			Expect(err).ToNot(HaveOccurred())
			Expect(service["put_key1"]).To(Equal("put_value"))
			Expect(service).ToNot(HaveKey("username"))
		})

		It("returns a 404 when the namespace does not exist", func() {
			response, err := env.Curl("PUT", fmt.Sprintf("%s%s/namespaces/idontexist/services/%s",
				serverURL, api.Root, svc1), strings.NewReader(changeRequest))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})

		It("returns a 404 when the service does not exist", func() {
			response, err := env.Curl("PUT", fmt.Sprintf("%s%s/namespaces/%s/services/bogus",
				serverURL, api.Root, namespace), strings.NewReader(changeRequest))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})

		When("service is bound to an app", func() {
			var app1 string
			BeforeEach(func() {
				app1 = catalog.NewAppName()

				env.MakeContainerImageApp(app1, 1, containerImageURL)
				env.BindAppService(app1, svc1, namespace)
			})

			AfterEach(func() {
				env.DeleteApp(app1)
			})

			Describe("workload restarts", func() {
				It("only restarts the app if the service has changed", func() {
					getPodNames := func(namespace, app string) ([]string, error) {
						podName, err := helpers.Kubectl("get", "pods", "-n", namespace, "-l", fmt.Sprintf("app.kubernetes.io/name=%s", app), "-o", "jsonpath='{.items[*].metadata.name}'")
						return strings.Split(podName, " "), err
					}

					oldPodNames, err := getPodNames(namespace, app1)
					Expect(err).ToNot(HaveOccurred())

					response, err := env.Curl("PUT", fmt.Sprintf("%s%s/namespaces/%s/services/%s",
						serverURL, api.Root, namespace, svc1), strings.NewReader(changeRequest))
					Expect(err).ToNot(HaveOccurred())
					Expect(response).ToNot(BeNil())
					Expect(response.StatusCode).To(Equal(http.StatusOK))

					var newPodNames []string
					// Wait until only one pod exists (restart is finished)
					Eventually(func() int {
						newPodNames, err = getPodNames(namespace, app1)
						Expect(err).ToNot(HaveOccurred())
						return len(newPodNames)
					}, "1m").Should(Equal(1))
					Expect(newPodNames).NotTo(Equal(oldPodNames))

					// Now try with no changes
					oldPodNames, err = getPodNames(namespace, app1)
					Expect(err).ToNot(HaveOccurred())

					response, err = env.Curl("PUT", fmt.Sprintf("%s%s/namespaces/%s/services/%s",
						serverURL, api.Root, namespace, svc1), strings.NewReader(changeRequest))
					Expect(err).ToNot(HaveOccurred())
					Expect(response).ToNot(BeNil())
					Expect(response.StatusCode).To(Equal(http.StatusOK))

					Consistently(func() []string {
						newPodNames, err := getPodNames(namespace, app1)
						Expect(err).ToNot(HaveOccurred())
						return newPodNames
					}, "10s").Should(Equal(oldPodNames))
				})
			})
		})
	})
})
