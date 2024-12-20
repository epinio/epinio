// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/domain"
	"github.com/epinio/epinio/internal/routes"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppUpdate Endpoint", LApplication, func() {
	var (
		namespace, containerImageURL string
	)

	BeforeEach(func() {
		containerImageURL = "epinio/sample-app"
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		DeferCleanup(func() {
			env.DeleteNamespace(namespace)
		})
	})

	When("instances is valid integer", func() {
		It("updates an application with the desired number of instances", func() {
			app := catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
			defer env.DeleteApp(app)

			appObj := appShow(namespace, app)
			Expect(appObj.Workload.Status).To(Equal("1/1"))

			request := map[string]interface{}{"instances": 3}
			_, statusCode := appUpdate(namespace, app, toJSON(request))
			Expect(statusCode).To(Equal(http.StatusOK))

			Eventually(func() string {
				return appShow(namespace, app).Workload.Status
			}, "1m").Should(Equal("3/3"))
		})
	})

	When("instances is invalid", func() {
		It("returns BadRequest when instances is a negative number", func() {
			app := catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
			defer env.DeleteApp(app)
			Expect(appShow(namespace, app).Workload.Status).To(Equal("1/1"))

			request := map[string]interface{}{"instances": -3}
			updateResponseBody, statusCode := appUpdate(namespace, app, toJSON(request))
			Expect(statusCode).To(Equal(http.StatusBadRequest))

			var errorResponse apierrors.ErrorResponse
			err := json.Unmarshal(updateResponseBody, &errorResponse)
			Expect(err).ToNot(HaveOccurred())
			Expect(errorResponse.Errors[0].Status).To(Equal(http.StatusBadRequest))
			Expect(errorResponse.Errors[0].Title).To(Equal("instances param should be integer equal or greater than zero"))
		})

		It("returns BadRequest when instances is not a number", func() {
			// The bad request does not even reach deeper validation, as it fails to
			// convert into the expected structure.

			app := catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
			defer env.DeleteApp(app)
			Expect(appShow(namespace, app).Workload.Status).To(Equal("1/1"))

			request := map[string]string{"instances": "not-a-number"}
			updateResponseBody, status := appUpdate(namespace, app, toJSON(request))
			Expect(status).To(Equal(http.StatusBadRequest))

			var errorResponse apierrors.ErrorResponse
			err := json.Unmarshal(updateResponseBody, &errorResponse)
			Expect(err).ToNot(HaveOccurred())
			Expect(errorResponse.Errors[0].Status).To(Equal(http.StatusBadRequest))
			Expect(errorResponse.Errors[0].Title).To(Equal("json: cannot unmarshal string into Go struct field ApplicationUpdateRequest.instances of type int32"))
		})
	})
	When("routes have changed", func() {
		// removes empty strings from the given slice
		deleteEmpty := func(elements []string) []string {
			var result []string
			for _, e := range elements {
				if e != "" {
					result = append(result, e)
				}
			}
			return result
		}

		checkCertificateDNSNames := func(appName, namespaceName string, routes ...string) {
			Eventually(func() int {
				out, err := proc.Kubectl("get", "certificates",
					"-n", namespaceName,
					"--selector", "app.kubernetes.io/name="+appName,
					"-o", "jsonpath={.items[*].spec.dnsNames[*]}")
				Expect(err).ToNot(HaveOccurred(), out)
				return len(deleteEmpty(strings.Split(out, " ")))
			}, "20s", "1s").Should(Equal(len(routes)))

			out, err := proc.Kubectl("get", "certificates",
				"-n", namespaceName,
				"--selector", "app.kubernetes.io/name="+appName,
				"-o", "jsonpath={.items[*].spec.dnsNames[*]}")
			Expect(err).ToNot(HaveOccurred(), out)
			certDomains := deleteEmpty(strings.Split(strings.TrimSpace(out), " "))
			Expect(certDomains).To(ContainElements(routes))
			Expect(len(certDomains)).To(Equal(len(routes)))
		}

		checkIngresses := func(appName, namespaceName string, routesStr ...string) {
			GinkgoHelper()

			routeObjects := []routes.Route{}
			for _, route := range routesStr {
				routeObjects = append(routeObjects, routes.FromString(route))
			}

			Eventually(func() int {
				out, err := proc.Kubectl("get", "ingresses",
					"-n", namespaceName,
					"--selector", "app.kubernetes.io/name="+appName,
					"-o", "jsonpath={.items[*].spec.rules[*].host}")
				Expect(err).ToNot(HaveOccurred(), out)
				return len(deleteEmpty(strings.Split(out, " ")))
			}, "20s", "1s").Should(Equal(len(routeObjects)))

			out, err := proc.Kubectl("get", "ingresses",
				"-n", namespaceName,
				"--selector", "app.kubernetes.io/name="+appName,
				"-o", "jsonpath={range .items[*]}{@.spec.rules[0].host}{@.spec.rules[0].http.paths[0].path} ")
			Expect(err).ToNot(HaveOccurred(), out)
			ingressRoutes := deleteEmpty(strings.Split(strings.TrimSpace(out), " "))
			trimmedRoutes := []string{}
			for _, ir := range ingressRoutes {
				trimmedRoutes = append(trimmedRoutes, strings.TrimSuffix(ir, "/"))
			}
			Expect(trimmedRoutes).To(ContainElements(routesStr))
			Expect(len(trimmedRoutes)).To(Equal(len(routesStr)))
		}

		// Checks if every secret referenced in a certificate of the given app,
		// has a corresponding secret. routes are used to wait until all
		// certificates are created.
		checkSecretsForCerts := func(appName, namespaceName string, routes ...string) {
			GinkgoHelper()

			Eventually(func() int {
				out, err := proc.Kubectl("get", "certificates",
					"-n", namespaceName,
					"--selector", "app.kubernetes.io/name="+appName,
					"-o", "jsonpath={.items[*].spec.secretName}")
				Expect(err).ToNot(HaveOccurred(), out)
				certSecrets := deleteEmpty(strings.Split(strings.TrimSpace(out), " "))
				return len(certSecrets)
			}, "20s", "1s").Should(Equal(len(routes)))

			out, err := proc.Kubectl("get", "certificates",
				"-n", namespaceName,
				"--selector", "app.kubernetes.io/name="+appName,
				"-o", "jsonpath={.items[*].spec.secretName}")
			Expect(err).ToNot(HaveOccurred(), out)
			certSecrets := deleteEmpty(strings.Split(strings.TrimSpace(out), " "))

			Eventually(func() []string {
				out, err = proc.Kubectl("get", "secrets", "-n", namespaceName, "-o", "jsonpath={.items[*].metadata.name}")
				Expect(err).ToNot(HaveOccurred(), out)
				existingSecrets := deleteEmpty(strings.Split(strings.TrimSpace(out), " "))
				return existingSecrets
			}, "60s", "1s").Should(ContainElements(certSecrets))
		}

		checkRoutesOnApp := func(appName, namespaceName string, routes ...string) {
			GinkgoHelper()

			out, err := proc.Kubectl("get", "apps", "-n", namespaceName, appName, "-o", "jsonpath={.spec.routes[*]}")
			Expect(err).ToNot(HaveOccurred(), out)
			appRoutes := deleteEmpty(strings.Split(strings.TrimSpace(out), " "))

			if appRoutes == nil {
				appRoutes = []string{}
			}

			Expect(appRoutes).To(Equal(routes))
		}

		It("synchronizes the ingresses of the application with the new routes list", func() {
			app := catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
			defer env.DeleteApp(app)

			defaultRoute, err := domain.AppDefaultRoute(context.Background(), app, namespace)
			Expect(err).ToNot(HaveOccurred())

			checkRoutesOnApp(app, namespace, defaultRoute)
			checkIngresses(app, namespace, defaultRoute)
			checkCertificateDNSNames(app, namespace, defaultRoute)
			checkSecretsForCerts(app, namespace, defaultRoute)

			appObj := appShow(namespace, app)
			Expect(appObj.Workload.Status).To(Equal("1/1"))

			newRoutes := []string{"domain1.org", "domain2.org"}
			data, err := json.Marshal(models.ApplicationUpdateRequest{
				Routes: newRoutes,
			})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("PATCH",
				fmt.Sprintf("%s%s/namespaces/%s/applications/%s",
					serverURL, v1.Root, namespace, app),
				strings.NewReader(string(data)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			checkRoutesOnApp(app, namespace, newRoutes...)
			checkIngresses(app, namespace, newRoutes...)
			checkCertificateDNSNames(app, namespace, newRoutes...)
			checkSecretsForCerts(app, namespace, newRoutes...)
		})

		It("synchronizes the ingresses of the application with a new empty routes list", func() {
			app := catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
			defer env.DeleteApp(app)

			defaultRoute, err := domain.AppDefaultRoute(context.Background(), app, namespace)
			Expect(err).ToNot(HaveOccurred())

			checkRoutesOnApp(app, namespace, defaultRoute)
			checkIngresses(app, namespace, defaultRoute)
			checkCertificateDNSNames(app, namespace, defaultRoute)
			checkSecretsForCerts(app, namespace, defaultRoute)

			appObj := appShow(namespace, app)
			Expect(appObj.Workload.Status).To(Equal("1/1"))

			newRoutes := []string{}
			data, err := json.Marshal(models.ApplicationUpdateRequest{
				Routes: newRoutes,
			})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("PATCH",
				fmt.Sprintf("%s%s/namespaces/%s/applications/%s",
					serverURL, v1.Root, namespace, app),
				strings.NewReader(string(data)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			checkRoutesOnApp(app, namespace, newRoutes...)
			checkIngresses(app, namespace, newRoutes...)
			checkCertificateDNSNames(app, namespace, newRoutes...)
			checkSecretsForCerts(app, namespace, newRoutes...)
		})
	})

	Describe("configuration bindings", func() {
		var (
			app                           string
			configuration, configuration2 string
		)

		BeforeEach(func() {
			app = catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
			configuration = catalog.NewConfigurationName()
			env.MakeConfiguration(configuration)
			configuration2 = catalog.NewConfigurationName()
			env.MakeConfiguration(configuration2)
		})

		AfterEach(func() {
			env.DeleteApp(app)
			env.DeleteConfigurations(configuration)
			env.DeleteConfigurations(configuration2)
		})

		// helper function to allow deterministic string comparison
		sortStrings := func(strings []string) []string {
			sort.Strings(strings)
			return strings
		}

		readConfigurationBindings := func(namespace, app string) []string {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications/%s",
				serverURL, v1.Root, namespace, app), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			var data models.App
			err = json.Unmarshal(bodyBytes, &data)
			Expect(err).ToNot(HaveOccurred())
			return data.Configuration.Configurations
		}

		It("binds a configuration to an app", func() {
			configurationBindings := readConfigurationBindings(namespace, app)
			Expect(configurationBindings).To(Equal([]string{}))

			newConfigurationBinding := []string{configuration}
			data, err := json.Marshal(models.ApplicationUpdateRequest{
				Configurations: newConfigurationBinding,
			})
			Expect(err).ToNot(HaveOccurred())
			response, err := env.Curl("PATCH",
				fmt.Sprintf("%s%s/namespaces/%s/applications/%s",
					serverURL, v1.Root, namespace, app),
				strings.NewReader(string(data)))

			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			configurationBindings = readConfigurationBindings(namespace, app)
			Expect(configurationBindings).To(Equal([]string{configuration}))
		})

		It("unbinds a configuration from an app", func() {
			env.BindAppConfiguration(app, configuration, namespace)
			env.BindAppConfiguration(app, configuration2, namespace)

			configurationBindings := readConfigurationBindings(namespace, app)
			Expect(configurationBindings).To(Equal(sortStrings([]string{configuration, configuration2})))

			// delete a single configuration by only providing one of the two bound configurations
			newConfigurationBinding := []string{configuration2}
			data, err := json.Marshal(models.ApplicationUpdateRequest{
				Configurations: newConfigurationBinding,
			})
			Expect(err).ToNot(HaveOccurred())
			response, err := env.Curl("PATCH",
				fmt.Sprintf("%s%s/namespaces/%s/applications/%s",
					serverURL, v1.Root, namespace, app),
				strings.NewReader(string(data)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			configurationBindings = readConfigurationBindings(namespace, app)
			Expect(configurationBindings).To(Equal([]string{configuration2}))

			// delete all configurations by providing an empty array
			env.BindAppConfiguration(app, configuration, namespace)
			configurationBindings = readConfigurationBindings(namespace, app)
			Expect(configurationBindings).To(Equal(sortStrings([]string{configuration, configuration2})))

			newConfigurationBinding = []string{}
			data, err = json.Marshal(models.ApplicationUpdateRequest{
				Configurations: newConfigurationBinding,
			})
			Expect(err).ToNot(HaveOccurred())
			response, err = env.Curl("PATCH",
				fmt.Sprintf("%s%s/namespaces/%s/applications/%s",
					serverURL, v1.Root, namespace, app),
				strings.NewReader(string(data)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			configurationBindings = readConfigurationBindings(namespace, app)
			Expect(configurationBindings).To(Equal([]string{}))
		})

		It("fails on non existing configuration bindings and does not touch any existing configuration config", func() {
			env.BindAppConfiguration(app, configuration, namespace)

			configurationBindings := readConfigurationBindings(namespace, app)
			Expect(configurationBindings).To(Equal(sortStrings([]string{configuration})))

			newConfigurationBinding := []string{"does_not_exist"}
			data, err := json.Marshal(models.ApplicationUpdateRequest{
				Configurations: newConfigurationBinding,
			})
			Expect(err).ToNot(HaveOccurred())
			response, err := env.Curl("PATCH",
				fmt.Sprintf("%s%s/namespaces/%s/applications/%s",
					serverURL, v1.Root, namespace, app),
				strings.NewReader(string(data)))
			Expect(err).ToNot(HaveOccurred())

			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			var errorResponse apierrors.ErrorResponse
			err = json.Unmarshal(bodyBytes, &errorResponse)
			Expect(err).ToNot(HaveOccurred())

			Expect(errorResponse.Errors[0].Status).To(Equal(http.StatusNotFound))
			Expect(errorResponse.Errors[0].Title).To(Equal("configuration 'does_not_exist' does not exist"))

			configurationBindings = readConfigurationBindings(namespace, app)
			Expect(configurationBindings).To(Equal([]string{configuration}))
		})
	})
})
