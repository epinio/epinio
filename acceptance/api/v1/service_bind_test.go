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
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers"
	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ServiceBind Endpoint", LService, func() {
	var namespace, containerImageURL string
	var catalogService models.CatalogService

	BeforeEach(func() {
		containerImageURL = "epinio/sample-app"

		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		catalogService = models.CatalogService{
			Meta: models.MetaLite{
				Name: catalog.NewCatalogServiceName(),
			},
			HelmChart: "nginx",
			HelmRepo: models.HelmRepo{
				Name: "",
				URL:  "https://charts.bitnami.com/bitnami",
			},
			Values: "{'service': {'type': 'ClusterIP'}}",
		}
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	When("the service doesn't exist", func() {
		var app string

		BeforeEach(func() {
			// Let's create an app so that only the service is missing
			app = catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
		})

		AfterEach(func() {
			env.DeleteApp(app)
		})

		It("returns 404", func() {
			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBind", namespace, "bogus"))
			requestBody, err := json.Marshal(models.ServiceBindRequest{AppName: app})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	When("the application doesn't exist", func() {
		var serviceName string

		BeforeEach(func() {
			catalog.CreateCatalogService(catalogService)

			// Let's create a service so that only app is missing
			serviceName = catalog.NewServiceName()
			catalog.CreateService(serviceName, namespace, catalogService)
		})

		AfterEach(func() {
			catalog.DeleteService(serviceName, namespace)
			catalog.DeleteCatalogService(catalogService.Meta.Name)
		})

		It("returns 404", func() {
			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBind", namespace, serviceName))
			requestBody, err := json.Marshal(models.ServiceBindRequest{AppName: "bogus"})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	When("both app and service exist", func() {
		var app, serviceName, chartName string

		BeforeEach(func() {
			// Use a chart that creates some secret (nginx doesn't)
			catalogService.HelmChart = "mysql"
			catalogService.Values = ""
			catalog.CreateCatalogService(catalogService)

			app = catalog.NewAppName()
			serviceName = catalog.NewServiceName()
			chartName = names.ServiceReleaseName(serviceName)

			env.MakeContainerImageApp(app, 1, containerImageURL)
			catalog.CreateService(serviceName, namespace, catalogService)
		})

		AfterEach(func() {
			env.DeleteApp(app)
			catalog.DeleteService(serviceName, namespace)
			catalog.DeleteCatalogService(catalogService.Meta.Name)
		})

		It("binds the service's secrets", func() {
			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBind", namespace, serviceName))
			requestBody, err := json.Marshal(models.ServiceBindRequest{AppName: app})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			matchString := fmt.Sprintf("Bound Configurations.*%s", chartName)
			Expect(appShowOut).To(MatchRegexp(matchString))
		})
	})

	When("app exist", func() {
		var app, serviceName, chartName string

		BeforeEach(func() {
			app = catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)

			// Use a chart that creates some secret (nginx doesn't)
			catalogService.HelmChart = "mysql"
			catalogService.Values = ""

			serviceName = catalog.NewServiceName()
			chartName = names.ServiceReleaseName(serviceName)
		})

		AfterEach(func() {
			env.DeleteApp(app)
		})

		When("service exist", func() {

			BeforeEach(func() {
				catalog.CreateCatalogService(catalogService)
				catalog.CreateService(serviceName, namespace, catalogService)
			})

			AfterEach(func() {
				catalog.DeleteService(serviceName, namespace)
				catalog.DeleteCatalogService(catalogService.Meta.Name)
			})

			It("binds the service's secrets", func() {
				endpoint := fmt.Sprintf("%s%s/%s",
					serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBind", namespace, serviceName))
				requestBody, err := json.Marshal(models.ServiceBindRequest{AppName: app})
				Expect(err).ToNot(HaveOccurred())

				response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				appShowOut, err := env.Epinio("", "app", "show", app)
				Expect(err).ToNot(HaveOccurred())
				matchString := fmt.Sprintf("Bound Configurations.*%s", chartName)
				Expect(appShowOut).To(MatchRegexp(matchString))
			})
		})

		When("service exist, and the catalog service has secret types defined", func() {
			var basicAuthSecretName, customSecretName string

			BeforeEach(func() {
				// Use a chart that creates some secret (nginx doesn't)
				catalogService.SecretTypes = []string{"Opaque", "BasicAuth"}

				catalog.CreateCatalogService(catalogService)
				catalog.CreateService(serviceName, namespace, catalogService)

				// create other secrets
				basicAuthSecretName = chartName + "-basic-secret"
				createSecret(basicAuthSecretName, namespace, chartName, "BasicAuth")

				customSecretName = chartName + "-custom-secret"
				createSecret(customSecretName, namespace, chartName, "Custom")
			})

			AfterEach(func() {
				catalog.DeleteService(serviceName, namespace)
				catalog.DeleteCatalogService(catalogService.Meta.Name)

				out, err := proc.Kubectl("delete", "secret", "-n", namespace, basicAuthSecretName)
				Expect(err).ToNot(HaveOccurred(), out)

				out, err = proc.Kubectl("delete", "secret", "-n", namespace, customSecretName)
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("binds the only service's secrets with the defined types", func() {
				endpoint := fmt.Sprintf("%s%s/%s",
					serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBind", namespace, serviceName))
				requestBody, err := json.Marshal(models.ServiceBindRequest{AppName: app})
				Expect(err).ToNot(HaveOccurred())

				response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				appShowOut, err := env.Epinio("", "app", "show", app)
				Expect(err).ToNot(HaveOccurred())

				Expect(appShowOut).To(MatchRegexp(chartName + "-mysql"))
				Expect(appShowOut).To(MatchRegexp(basicAuthSecretName))
				Expect(appShowOut).To(Not(MatchRegexp(customSecretName)))
			})

		})
	})
})

func createSecret(name, namespace, serviceName string, secretType corev1.SecretType) {
	secret := &corev1.Secret{
		Type: secretType,
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/instance": serviceName,
			},
		},
		StringData: map[string]string{
			"username": "username",
			"password": "password",
		},
	}

	jsonBytes, err := json.Marshal(secret)
	Expect(err).ToNot(HaveOccurred())

	filePath, err := helpers.CreateTmpFile(string(jsonBytes))
	Expect(err).ToNot(HaveOccurred())

	out, err := proc.Kubectl("apply", "-f", filePath)
	Expect(err).ToNot(HaveOccurred(), out)
}
