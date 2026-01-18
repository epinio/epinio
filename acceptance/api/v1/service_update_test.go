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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceUpdate Endpoint", LService, func() {
	var namespace, containerImageURL, app, serviceName, chartName string
	var catalogService models.CatalogService

	getPodNames := func(namespace, app string) ([]string, error) {
		podName, err := proc.Kubectl("get", "pods", "-n", namespace, "-l", fmt.Sprintf("app.kubernetes.io/name=%s", app), "-o", "jsonpath='{.items[*].metadata.name}'")
		return strings.Split(strings.Trim(podName, "'"), " "), err
	}

	BeforeEach(func() {
		containerImageURL = "epinio/sample-app"

		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		catalogService = models.CatalogService{
			Meta: models.MetaLite{
				Name: catalog.NewCatalogServiceName(),
			},
			HelmChart: "mysql",
			HelmRepo: models.HelmRepo{
				Name: "",
				URL:  "https://charts.bitnami.com/bitnami",
			},
		}

		catalog.CreateCatalogService(catalogService)

		app = catalog.NewAppName()
		env.MakeContainerImageApp(app, 1, containerImageURL)

		serviceName = catalog.NewServiceName()
		chartName = names.ServiceReleaseName(serviceName)

		catalog.CreateService(serviceName, namespace, catalogService)

		// Bind the service to the app
		out, err := env.Epinio("", "service", "bind", serviceName, app)
		Expect(err).ToNot(HaveOccurred(), out)

		// Wait for app to settle
		Eventually(func() string {
			out, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			return out
		}, "2m").Should(ContainSubstring("1/1"))
	})

	AfterEach(func() {
		env.DeleteApp(app)
		catalog.DeleteService(serviceName, namespace)
		catalog.DeleteCatalogService(catalogService.Meta.Name)
		env.DeleteNamespace(namespace)
	})

	When("restart parameter is provided", func() {
		It("does not restart bound apps when restart is false", func() {
			By("getting pod names before update")
			oldPodNames, err := getPodNames(namespace, app)
			Expect(err).ToNot(HaveOccurred())

			By("updating service with restart: false")
			restartFalse := false
			request := models.ServiceUpdateRequest{
				Set: map[string]string{
					"testkey": "testvalue",
				},
				Restart: &restartFalse,
			}
			requestBody, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())

			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceUpdate", namespace, serviceName))
			response, err := env.Curl("PATCH", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("verifying pods DID NOT restart")
			Consistently(func() []string {
				names, err := getPodNames(namespace, app)
				Expect(err).ToNot(HaveOccurred())
				return names
			}, "15s", "2s").Should(ContainElements(oldPodNames))

			By("verifying app is still healthy")
			out, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("1/1"))
		})

		It("restarts bound apps when restart is true", func() {
			By("getting pod names before update")
			oldPodNames, err := getPodNames(namespace, app)
			Expect(err).ToNot(HaveOccurred())

			By("updating service with restart: true")
			restartTrue := true
			request := models.ServiceUpdateRequest{
				Set: map[string]string{
					"testkey": "testvalue-restart",
				},
				Restart: &restartTrue,
			}
			requestBody, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())

			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceUpdate", namespace, serviceName))
			response, err := env.Curl("PATCH", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("verifying pods DID restart")
			Eventually(func() []string {
				names, err := getPodNames(namespace, app)
				Expect(err).ToNot(HaveOccurred())
				return names
			}, "2m", "2s").ShouldNot(ContainElements(oldPodNames))

			By("verifying app is healthy after restart")
			Eventually(func() string {
				out, err := env.Epinio("", "app", "show", app)
				Expect(err).ToNot(HaveOccurred())
				return out
			}, "2m").Should(ContainSubstring("1/1"))
		})

		It("restarts bound apps by default when restart parameter is omitted (backward compatibility)", func() {
			By("getting pod names before update")
			oldPodNames, err := getPodNames(namespace, app)
			Expect(err).ToNot(HaveOccurred())

			By("updating service WITHOUT restart parameter (default behavior)")
			request := models.ServiceUpdateRequest{
				Set: map[string]string{
					"testkey": "testvalue-default",
				},
				// Restart is nil - should default to true
			}
			requestBody, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())

			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceUpdate", namespace, serviceName))
			response, err := env.Curl("PATCH", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("verifying pods DID restart (default behavior)")
			Eventually(func() []string {
				names, err := getPodNames(namespace, app)
				Expect(err).ToNot(HaveOccurred())
				return names
			}, "2m", "2s").ShouldNot(ContainElements(oldPodNames))

			By("verifying app is healthy after restart")
			Eventually(func() string {
				out, err := env.Epinio("", "app", "show", app)
				Expect(err).ToNot(HaveOccurred())
				return out
			}, "2m").Should(ContainSubstring("1/1"))
		})
	})

	It("returns 404 when service does not exist", func() {
		request := models.ServiceUpdateRequest{
			Set: map[string]string{
				"testkey": "testvalue",
			},
		}
		requestBody, err := json.Marshal(request)
		Expect(err).ToNot(HaveOccurred())

		endpoint := fmt.Sprintf("%s%s/%s",
			serverURL, apiv1.Root, apiv1.Routes.Path("ServiceUpdate", namespace, "nonexistent-service"))
		response, err := env.Curl("PATCH", endpoint, strings.NewReader(string(requestBody)))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound))
	})

	// Suppress unused variable warning - chartName is used for documentation/debugging
	_ = chartName
})

