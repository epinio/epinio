package acceptance_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Services API Application Endpoints, Mutations", func() {
	var org string

	BeforeEach(func() {
		org = newOrgName()
		setupAndTargetOrg(org)
		setupInClusterServices()
	})

	Describe("POST api/v1/orgs/:org/services/", func() {
		var service string

		BeforeEach(func() {
			service = newServiceName()
		})

		It("returns a 'bad request' for a non JSON body", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/services",
					serverURL, org),
				strings.NewReader(``))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("unexpected end of JSON input\n"))
		})

		It("returns a 'bad request' for a non-object JSON body", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/services",
					serverURL, org),
				strings.NewReader(`[]`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("json: cannot unmarshal array into Go value of type models.CatalogCreateRequest\n"))
		})

		It("returns a 'bad request' for JSON object without `name` key", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/services",
					serverURL, org),
				strings.NewReader(`{}`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Cannot create service without a name\n"))
		})

		It("returns a 'bad request' for JSON object without `class` key", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/services",
					serverURL, org),
				strings.NewReader(`{
				    "name": "meh"
				}`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Cannot create service without a service class\n"))
		})

		It("returns a 'bad request' for JSON object without `plan` key", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/services",
					serverURL, org),
				strings.NewReader(`{
				    "name": "meh",
				    "class": "meh"
				}`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Cannot create service without a service plan\n"))
		})

		It("returns a 'not found' when the org does not exist", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/bogus/services",
					serverURL),
				strings.NewReader(`{
				    "name": "meh",
				    "class": "meh",
				    "plan": "meh",
				    "waitforprovision": true
				}`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Organization 'bogus' does not exist\n"))
		})

		Context("with conflicting service", func() {
			var service string

			BeforeEach(func() {
				service = newServiceName()
				makeCustomService(service)
			})

			AfterEach(func() {
				cleanupService(service)
			})

			It("returns a 'conflict'", func() {
				response, err := Curl("POST",
					fmt.Sprintf("%s/api/v1/orgs/%s/services",
						serverURL, org),
					strings.NewReader(fmt.Sprintf(`{
					    "name": "%s",
					    "class": "meh",
					    "plan": "meh",
					    "waitforprovision": true
					}`, service)))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusConflict), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal("Service '" + service + "' already exists\n"))
			})
		})

		It("returns a 'not found' when the class does not exist", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/services",
					serverURL, org),
				strings.NewReader(fmt.Sprintf(`{
				    "name": "%s",
				    "class": "meh",
				    "plan": "meh",
				    "waitforprovision": true
				}`, service)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Service class 'meh' does not exist\n"))
		})

		It("returns a 'not found' when the plan does not exist", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/services",
					serverURL, org),
				strings.NewReader(fmt.Sprintf(`{
				    "name": "%s",
				    "class": "mariadb",
				    "plan": "meh",
				    "waitforprovision": true
				}`, service)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Service plan 'meh' does not exist for class 'mariadb'\n"))
		})

		Describe("Creation", func() {
			AfterEach(func() {
				cleanupService(service)
			})

			It("creates the catalog service and waits for it to be provisioned", func() {
				response, err := Curl("POST",
					fmt.Sprintf("%s/api/v1/orgs/%s/services",
						serverURL, org),
					strings.NewReader(fmt.Sprintf(`{
					    "name": "%s",
					    "class": "mariadb",
					    "plan": "10-3-22",
					    "waitforprovision": true
					}`, service)))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(""))

				out, err := Epinio("service list", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp(service))
			})

			It("creates the catalog service and returns immediately", func() {
				response, err := Curl("POST",
					fmt.Sprintf("%s/api/v1/orgs/%s/services",
						serverURL, org),
					strings.NewReader(fmt.Sprintf(`{
					    "name": "%s",
					    "class": "mariadb",
					    "plan": "10-3-22",
					    "waitforprovision": false
					}`, service)))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(""))

				// Explicit wait in the test itself for the service to be provisioned/appear.
				// This takes the place of the `service list` command in the previous test,
				// which simply checks for presence.
				Eventually(func() string {
					out, err := Epinio("service show "+service, "")
					Expect(err).ToNot(HaveOccurred(), out)
					return out
				}, "5m").Should(MatchRegexp(`Status .*\|.* Provisioned`))
			})
		})
	})

	Describe("POST api/v1/orgs/:org/custom-services/", func() {
		It("returns a 'bad request' for a non JSON body", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/custom-services",
					serverURL, org),
				strings.NewReader(``))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("unexpected end of JSON input\n"))
		})

		It("returns a 'bad request' for a non-object JSON body", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/custom-services",
					serverURL, org),
				strings.NewReader(`[]`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("json: cannot unmarshal array into Go value of type models.CustomCreateRequest\n"))
		})

		It("returns a 'bad request' for JSON object without `name` key", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/custom-services",
					serverURL, org),
				strings.NewReader(`{}`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Cannot create custom service without a name\n"))
		})

		It("returns a 'bad request' for JSON object empty `data` key", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/custom-services",
					serverURL, org),
				strings.NewReader(`{
				    "name": "meh"
				}`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Cannot create custom service without data\n"))
		})

		It("returns a 'not found' when the org does not exist", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/bogus/custom-services",
					serverURL),
				strings.NewReader(`{
				    "name": "meh",
				    "data": {"host":"localhost", "port":"9999"}
				}`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Organization 'bogus' does not exist\n"))
		})

		Context("with conflicting service", func() {
			var service string

			BeforeEach(func() {
				service = newServiceName()
				makeCustomService(service)
			})

			AfterEach(func() {
				cleanupService(service)
			})

			It("returns a 'conflict'", func() {
				response, err := Curl("POST",
					fmt.Sprintf("%s/api/v1/orgs/%s/custom-services",
						serverURL, org),
					strings.NewReader(fmt.Sprintf(`{
					    "name": "%s",
					    "data": {"host":"localhost", "port":"9999"}
					}`, service)))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusConflict), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal("Service '" + service + "' already exists\n"))
			})
		})

		Describe("Creation", func() {
			var service string

			BeforeEach(func() {
				service = newServiceName()
			})

			AfterEach(func() {
				cleanupService(service)
			})

			It("creates the custom service", func() {
				response, err := Curl("POST",
					fmt.Sprintf("%s/api/v1/orgs/%s/custom-services",
						serverURL, org),
					strings.NewReader(fmt.Sprintf(`{
					    "name": "%s",
					    "data": {"host":"localhost", "port":"9999"}
					}`, service)))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(""))
			})
		})
	})

	Describe("DELETE api/v1/orgs/:org/services/:service", func() {
		var service string

		BeforeEach(func() {
			service = newServiceName()
		})

		It("returns a 'bad request' for a non JSON body", func() {
			response, err := Curl("DELETE",
				fmt.Sprintf("%s/api/v1/orgs/idontexist/services/%s",
					serverURL, service),
				strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("unexpected end of JSON input\n"))
		})

		It("returns a 'bad request' for a non-object JSON body", func() {
			response, err := Curl("DELETE",
				fmt.Sprintf("%s/api/v1/orgs/idontexist/services/%s",
					serverURL, service),
				strings.NewReader(`[]`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("json: cannot unmarshal array into Go value of type models.DeleteRequest\n"))
		})

		It("returns a 'not found' when the org does not exist", func() {
			response, err := Curl("DELETE",
				fmt.Sprintf("%s/api/v1/orgs/idontexist/services/%s",
					serverURL, service),
				strings.NewReader(`{ "unbind": false }`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Organization 'idontexist' does not exist\n"))
		})

		It("returns a 'not found' when the service does not exist", func() {
			response, err := Curl("DELETE",
				fmt.Sprintf("%s/api/v1/orgs/%s/services/bogus", serverURL, org),
				strings.NewReader(`{ "unbind": false }`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("service 'bogus' not found\n"))
		})

		Context("with bound applications", func() {
			var app string
			var service string

			BeforeEach(func() {
				service = newServiceName()
				app = newAppName()
				makeCustomService(service)
				makeApp(app)
				bindAppService(app, service, org)
			})

			AfterEach(func() {
				cleanupApp(app)
				cleanupService(service)
			})

			It("returns 'bad request'", func() {
				response, err := Curl("DELETE",
					fmt.Sprintf("%s/api/v1/orgs/%s/services/%s",
						serverURL, org, service),
					strings.NewReader(`{ "unbind": false }`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal("{\"boundapps\":[\"" + app + "\"]}\n"))
			})

			It("unbinds and removes the service, when former is requested", func() {
				response, err := Curl("DELETE",
					fmt.Sprintf("%s/api/v1/orgs/%s/services/%s",
						serverURL, org, service),
					strings.NewReader(`{ "unbind": true }`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal("{\"boundapps\":[\"" + app + "\"]}"))
			})
		})

		Context("without bound applications", func() {
			var service string

			BeforeEach(func() {
				service = newServiceName()
				makeCustomService(service)
			})

			It("removes the service", func() {
				response, err := Curl("DELETE",
					fmt.Sprintf("%s/api/v1/orgs/%s/services/%s",
						serverURL, org, service),
					strings.NewReader(`{ "unbind" : false }`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(bodyBytes)).To(Equal("{\"boundapps\":null}"))
			})
		})
	})

	Describe("POST api/v1/orgs/:org/applications/:arg/services/", func() {
		var app string

		BeforeEach(func() {
			app = newAppName()
		})

		It("returns a 'bad request' for a non JSON body", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s/services",
					serverURL, org, app),
				strings.NewReader(``))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("unexpected end of JSON input\n"))
		})

		It("returns a 'bad request' for a non-object JSON body", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s/services",
					serverURL, org, app),
				strings.NewReader(`[]`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("json: cannot unmarshal array into Go value of type models.BindRequest\n"))
		})

		It("returns a 'bad request' for JSON object without `name` key", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s/services", serverURL, org, app),
				strings.NewReader(`{}`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Cannot bind service without a name\n"))
		})

		It("returns a 'not found' when the org does not exist", func() {
			response, err := Curl("POST",
				fmt.Sprintf("%s/api/v1/orgs/bogus/applications/_dummy_/services", serverURL),
				strings.NewReader(`{ "name": "meh" }`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Organization 'bogus' does not exist\n"))
		})

		Context("with application", func() {
			var app string
			var service string

			BeforeEach(func() {
				app = newAppName()
				service = newServiceName()
				makeApp(app)
				makeCustomService(service)
			})

			AfterEach(func() {
				cleanupApp(app)
				cleanupService(service)
			})

			It("returns a 'not found' when the service does not exist", func() {
				response, err := Curl("POST",
					fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s/services",
						serverURL, org, app),
					strings.NewReader(`{ "name": "bogus" }`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal("service 'bogus' not found\n"))
			})

			Context("and already bound", func() {
				BeforeEach(func() {
					bindAppService(app, service, org)
				})

				It("returns a 'conflict'", func() {
					response, err := Curl("POST",
						fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s/services",
							serverURL, org, app),
						strings.NewReader(fmt.Sprintf(`{ "name": "%s" }`, service)))
					Expect(err).ToNot(HaveOccurred())
					Expect(response).ToNot(BeNil())

					defer response.Body.Close()
					bodyBytes, err := ioutil.ReadAll(response.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusConflict), string(bodyBytes))
					Expect(string(bodyBytes)).To(Equal("service '" + service + "' already bound\n"))
				})
			})

			It("binds the service", func() {
				response, err := Curl("POST",
					fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s/services",
						serverURL, org, app),
					strings.NewReader(fmt.Sprintf(`{ "name": "%s" }`, service)))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(""))
			})
		})
	})

	Describe("DELETE api/v1/orgs/:org/applications/:app/services/:service", func() {
		var app string
		var service string

		BeforeEach(func() {
			service = newServiceName()
			app = newAppName()
		})

		It("returns a 'not found' when the org does not exist", func() {
			response, err := Curl("DELETE",
				fmt.Sprintf("%s/api/v1/orgs/idontexist/applications/%s/services/%s",
					serverURL, app, service),
				strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("Organization 'idontexist' does not exist\n"))
		})

		It("returns a 'not found' when the application does not exist", func() {
			response, err := Curl("DELETE",
				fmt.Sprintf("%s/api/v1/orgs/%s/applications/bogus/services/%s",
					serverURL, org, service),
				strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal("application 'bogus' not found\n"))
		})

		Context("with application", func() {
			var app string

			BeforeEach(func() {
				app = newAppName()
				makeApp(app)
			})

			AfterEach(func() {
				cleanupApp(app)
			})

			It("returns a 'not found' when the service does not exist", func() {
				response, err := Curl("DELETE",
					fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s/services/bogus",
						serverURL, org, app),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal("service 'bogus' not found\n"))
			})

			Context("with service", func() {
				var service string

				BeforeEach(func() {
					service = newServiceName()
					makeCustomService(service)
				})

				AfterEach(func() {
					cleanupService(service)
				})

				Context("already bound", func() {
					BeforeEach(func() {
						bindAppService(app, service, org)
					})

					It("unbinds the service", func() {
						response, err := Curl("DELETE",
							fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s/services/%s",
								serverURL, org, app, service),
							strings.NewReader(""))
						Expect(err).ToNot(HaveOccurred())
						Expect(response).ToNot(BeNil())

						defer response.Body.Close()
						bodyBytes, err := ioutil.ReadAll(response.Body)
						Expect(err).ToNot(HaveOccurred())
						Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
						Expect(string(bodyBytes)).To(Equal(""))
					})
				})

				It("returns a 'bad request' when the service is not bound", func() {
					response, err := Curl("DELETE",
						fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s/services/%s",
							serverURL, org, app, service),
						strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					Expect(response).ToNot(BeNil())

					defer response.Body.Close()
					bodyBytes, err := ioutil.ReadAll(response.Body)
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(bodyBytes)).To(Equal("service '" + service + "' is not bound\n"))
				})
			})
		})
	})
})
