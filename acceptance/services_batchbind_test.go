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

package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const containerImageURL = "epinio/sample-app"

var _ = Describe("Service Batch Binding (CLI)", LService, func() {
	var (
		namespace string
		appName   string
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()
		env.MakeContainerImageApp(appName, 1, containerImageURL)
	})

	AfterEach(func() {
		env.DeleteApp(appName)
		env.DeleteNamespace(namespace)
	})

	Describe("Single Service Binding (Backward Compatibility)", func() {
		var (
			catalogService models.CatalogService
			service1       string
		)

		BeforeEach(func() {
			catalogService = catalog.NginxCatalogService(catalog.NewCatalogServiceName())
			service1 = catalog.NewServiceName()
			catalog.CreateService(service1, namespace, catalogService)
		})

		AfterEach(func() {
			catalog.DeleteService(service1, namespace)
		})

		It("binds a single service using old format: SERVICE APP", func() {
			// Old format: epinio service bind SERVICE APP
			out, err := env.Epinio("", "service", "bind", service1, appName)
			Expect(err).ToNot(HaveOccurred(), out)

			// Verify service is bound
			appInfo := env.ShowApp(appName, namespace)
			Expect(appInfo.Configuration.Services).To(ContainElement(service1))
		})
	})

	Describe("Multiple Service Binding (Batch)", func() {
		var (
			catalog1 models.CatalogService
			catalog2 models.CatalogService
			service1 string
			service2 string
			service3 string
		)

		BeforeEach(func() {
			catalog1 = catalog.NginxCatalogService(catalog.NewCatalogServiceName())
			catalog2 = catalog.RedisCatalogService(catalog.NewCatalogServiceName())

			service1 = catalog.NewServiceName()
			service2 = catalog.NewServiceName()
			service3 = catalog.NewServiceName()

			catalog.CreateService(service1, namespace, catalog1)
			catalog.CreateService(service2, namespace, catalog1)
			catalog.CreateService(service3, namespace, catalog2)
		})

		AfterEach(func() {
			catalog.DeleteService(service1, namespace)
			catalog.DeleteService(service2, namespace)
			catalog.DeleteService(service3, namespace)
		})

		It("binds multiple services using new batch format: APP SERVICE1 SERVICE2 SERVICE3", func() {
			// New batch format: epinio service bind APP SERVICE1 SERVICE2 SERVICE3
			out, err := env.Epinio("", "service", "bind", appName, service1, service2, service3)
			Expect(err).ToNot(HaveOccurred(), out)

			// Verify all services are bound
			appInfo := env.ShowApp(appName, namespace)
			Expect(appInfo.Configuration.Services).To(ConsistOf(service1, service2, service3))
		})

		It("binds two services in batch", func() {
			// New batch format with 2 services
			out, err := env.Epinio("", "service", "bind", appName, service1, service2)
			Expect(err).ToNot(HaveOccurred(), out)

			// Verify both services are bound
			appInfo := env.ShowApp(appName, namespace)
			Expect(appInfo.Configuration.Services).To(ConsistOf(service1, service2))
		})

		It("binds services from different catalogs", func() {
			// Mix nginx and redis services
			out, err := env.Epinio("", "service", "bind", appName, service1, service3)
			Expect(err).ToNot(HaveOccurred(), out)

			// Verify both services are bound
			appInfo := env.ShowApp(appName, namespace)
			Expect(appInfo.Configuration.Services).To(ConsistOf(service1, service3))
		})

		It("returns error when application doesn't exist", func() {
			nonExistentApp := "nonexistent-app"
			out, err := env.Epinio("", "service", "bind", nonExistentApp, service1, service2)
			Expect(err).To(HaveOccurred())
			// The API returns e.g. "application '<name>' does not exist" (404).
			// Accept both legacy and current wording.
			Expect(out).To(SatisfyAny(
				ContainSubstring("not found"),
				ContainSubstring("does not exist"),
			))
		})

		It("returns error when a service doesn't exist", func() {
			nonExistentService := "nonexistent-service"
			out, err := env.Epinio("", "service", "bind", appName, service1, nonExistentService)
			Expect(err).To(HaveOccurred())
			// The API returns: "service '<name>' does not exist" (404).
			// Keep a loose match to avoid brittleness across server/CLI versions.
			Expect(out).To(SatisfyAny(
				ContainSubstring("not found"),
				ContainSubstring("does not exist"),
			))
		})
	})

	Describe("Backward Compatibility Tests", func() {
		var (
			catalogService models.CatalogService
			service1       string
			service2       string
		)

		BeforeEach(func() {
			catalogService = catalog.NginxCatalogService(catalog.NewCatalogServiceName())
			service1 = catalog.NewServiceName()
			service2 = catalog.NewServiceName()
			catalog.CreateService(service1, namespace, catalogService)
			catalog.CreateService(service2, namespace, catalogService)
		})

		AfterEach(func() {
			catalog.DeleteService(service1, namespace)
			catalog.DeleteService(service2, namespace)
		})

		It("old format still works with 2 args (SERVICE APP)", func() {
			// Old format: SERVICE APP
			out, err := env.Epinio("", "service", "bind", service1, appName)
			Expect(err).ToNot(HaveOccurred(), out)

			appInfo := env.ShowApp(appName, namespace)
			Expect(appInfo.Configuration.Services).To(ContainElement(service1))
		})

		It("new format works with 3+ args (APP SERVICE1 SERVICE2...)", func() {
			// New format: APP SERVICE1 SERVICE2
			out, err := env.Epinio("", "service", "bind", appName, service1, service2)
			Expect(err).ToNot(HaveOccurred(), out)

			appInfo := env.ShowApp(appName, namespace)
			Expect(appInfo.Configuration.Services).To(ConsistOf(service1, service2))
		})

		It("can bind additional services after initial binding", func() {
			// First bind one service with old format
			out, err := env.Epinio("", "service", "bind", service1, appName)
			Expect(err).ToNot(HaveOccurred(), out)

			// Then bind another with new format.
			// Note: `epinio service bind` treats exactly 2 args as the old format
			// (SERVICE APP). Therefore use 3+ args here to force the batch mode.
			out, err = env.Epinio("", "service", "bind", appName, service1, service2)
			Expect(err).ToNot(HaveOccurred(), out)

			// Verify both are bound
			appInfo := env.ShowApp(appName, namespace)
			Expect(appInfo.Configuration.Services).To(ConsistOf(service1, service2))
		})
	})
})
