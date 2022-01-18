package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Namespaces API Application Endpoints", func() {
	var namespace string
	const jsOK = `{"status":"ok"}`
	containerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		// Wait for server to be up and running
		Eventually(func() error {
			_, err := env.Curl("GET", serverURL+api.Root+"/info", strings.NewReader(""))
			return err
		}, "1m").ShouldNot(HaveOccurred())
	})
	Context("Namespaces", func() {
		Describe("GET /api/v1/namespaces", func() {
			It("lists all namespaces", func() {
				response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				var namespaces models.NamespaceList
				err = json.Unmarshal(bodyBytes, &namespaces)
				Expect(err).ToNot(HaveOccurred())

				// See global BeforeEach for where this namespace is set up.
				Expect(namespaces).Should(ContainElements(models.Namespace{
					Name: namespace,
				}))
			})
			When("basic auth credentials are not provided", func() {
				It("returns a 401 response", func() {
					request, err := http.NewRequest("GET", fmt.Sprintf("%s%s/namespaces",
						serverURL, api.Root), strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					response, err := env.Client().Do(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})
		})

		Describe("POST /api/v1/namespaces", func() {
			It("fails for non JSON body", func() {
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(``))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody).To(HaveKey("errors"), string(bodyBytes))
				Expect(responseBody["errors"][0].Title).To(Equal("EOF"))
			})

			It("fails for non-object JSON body", func() {
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`[]`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(
					Equal("json: cannot unmarshal array into Go value of type models.NamespaceCreateRequest"))
			})

			It("fails for JSON object without name key", func() {
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`{}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(
					Equal("name of namespace to create not found"))
			})

			It("fails for a known namespace", func() {
				// Create the namespace
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`{"name":"birdy"}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(jsOK))

				// And the 2nd attempt should now fail
				By("creating the same namespace a second time")

				response, err = env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`{"name":"birdy"}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err = ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusConflict), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(
					Equal("Namespace 'birdy' already exists"))
			})

			It("fails for a restricted namespace", func() {
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`{"name":"epinio"}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(
					Equal("Namespace 'epinio' name cannot be used. Please try another name"))
			})

			It("creates a new namespace", func() {
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`{"name":"birdwatcher"}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(jsOK))
			})
		})

		Describe("GET /api/v1/namespaces/:namespace", func() {
			It("lists the namespace data", func() {
				response, err := env.Curl("GET",
					fmt.Sprintf("%s%s/namespaces/%s",
						serverURL, api.Root, namespace),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())

				var responseSpace models.Namespace
				err = json.Unmarshal(bodyBytes, &responseSpace)
				Expect(err).ToNot(HaveOccurred(), string(bodyBytes))
				Expect(responseSpace).To(Equal(models.Namespace{
					Name:     namespace,
					Apps:     nil,
					Services: nil,
				}))
			})
		})

		Describe("DELETE /api/v1/namespaces/:namespace", func() {
			It("deletes an namespace", func() {
				response, err := env.Curl("DELETE",
					fmt.Sprintf("%s%s/namespaces/%s",
						serverURL, api.Root, namespace),
					strings.NewReader(``))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(jsOK))

				_, err = proc.Kubectl("get", "namespace", namespace)
				Expect(err).To(HaveOccurred())
			})

			It("deletes an namespace including apps and services", func() {
				app1 := catalog.NewAppName()
				env.MakeContainerImageApp(app1, 1, containerImageURL)
				svc1 := catalog.NewServiceName()
				env.MakeService(svc1)
				env.BindAppService(app1, svc1, namespace)

				response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/namespaces/%s",
					serverURL, api.Root, namespace),
					strings.NewReader(``))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(bodyBytes)).To(Equal(jsOK))

				env.VerifyNamespaceNotExist(namespace)
			})
		})

		Describe("GET /api/v1/namespacematches", func() {
			It("lists all namespaces for empty prefix", func() {
				response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespacematches",
					serverURL, api.Root),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				resp := models.NamespacesMatchResponse{}
				err = json.Unmarshal(bodyBytes, &resp)
				Expect(err).ToNot(HaveOccurred())

				// See global BeforeEach for where this namespace is set up.
				Expect(resp.Names).Should(ContainElements(namespace))
			})
			It("lists no namespaces matching the prefix", func() {
				response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespacematches/bogus",
					serverURL, api.Root),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				resp := models.NamespacesMatchResponse{}
				err = json.Unmarshal(bodyBytes, &resp)
				Expect(err).ToNot(HaveOccurred())

				// See global BeforeEach for where this namespace is set up.
				Expect(resp.Names).Should(BeEmpty())
			})
			It("lists all namespaces matching the prefix", func() {
				response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespacematches/na",
					serverURL, api.Root),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				resp := models.NamespacesMatchResponse{}
				err = json.Unmarshal(bodyBytes, &resp)
				Expect(err).ToNot(HaveOccurred())

				// See global BeforeEach for where this namespace is set up.
				Expect(resp.Names).ShouldNot(BeEmpty())
			})
			When("basic auth credentials are not provided", func() {
				It("returns a 401 response", func() {
					request, err := http.NewRequest("GET", fmt.Sprintf("%s%s/namespacematches",
						serverURL, api.Root), strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					response, err := env.Client().Do(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})
		})
	})
})
