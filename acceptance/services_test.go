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
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const mysqlVersion = "8.0.31" // Doesn't change too often

var _ = Describe("Services", LService, func() {
	var catalogService models.CatalogService

	BeforeEach(func() {
		serviceName := catalog.NewCatalogServiceName()

		catalogService = models.CatalogService{
			Meta: models.MetaLite{
				Name: serviceName,
			},
			HelmChart: "nginx",
			HelmRepo: models.HelmRepo{
				Name: "",
				URL:  "https://charts.bitnami.com/bitnami",
			},
			Values: "{'service': {'type': 'ClusterIP'}}",
		}

		catalog.CreateCatalogService(catalogService)
	})

	AfterEach(func() {
		catalog.DeleteCatalogService(catalogService.Meta.Name)
	})

	Describe("Catalog", func() {

		It("lists the standard catalog", func() {
			out, err := env.Epinio("", "service", "catalog")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(ContainSubstring("Getting catalog"))
			Expect(out).To(
				HaveATable(
					WithHeaders("NAME", "CREATED", "VERSION", "DESCRIPTION"),
					WithRow("mysql-dev", WithDate(), ".*", ".*"),
					WithRow("postgresql-dev", WithDate(), ".*", ".*"),
					WithRow("rabbitmq-dev", WithDate(), ".*", ".*"),
					WithRow("redis-dev", WithDate(), ".*", ".*"),
					WithRow(catalogService.Meta.Name, WithDate(), "", ".*"),
				),
			)
		})

		It("lists the catalog details", func() {
			out, err := env.Epinio("", "service", "catalog", "redis-dev")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(ContainSubstring("Show service details"))
			Expect(out).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Name", "redis-dev"),
					WithRow("Created", WithDate()),
					WithRow("Version", "[0-9]+\\.[0-9]+\\.[0-9]+"),
					WithRow("Short Description", ".*"),
					WithRow("Description", ".*"),
				),
			)
		})

		When("Adding a catalog entry", func() {
			// Already added an nginx catalog service in the top level before block
			// It's meant to be used by other tests too to make tests faster, because
			// mysql takes more time to provision.

			It("lists the extended catalog", func() {
				out, err := env.Epinio("", "service", "catalog")
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(out).To(ContainSubstring("Getting catalog"))
				Expect(out).To(ContainSubstring(catalogService.Meta.Name))
			})

			It("lists the extended catalog details", func() {
				out, err := env.Epinio("", "service", "catalog", catalogService.Meta.Name)
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(out).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Name", catalogService.Meta.Name),
					),
				)
			})
		})

		Context("command completion", func() {
			It("matches empty prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "catalog", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(catalogService.Meta.Name))
			})

			It("does not match unknown prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "catalog", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "service", "catalog", catalogService.Meta.Name, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(catalogService.Meta.Name))
			})
		})
	})

	deleteServiceFromNamespace := func(namespace, service string) {
		env.TargetNamespace(namespace)
		env.DeleteService(service)
	}

	Describe("Show", func() {
		var namespace, service string

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			service = catalog.NewServiceName()

			By("create it")
			out, err := env.Epinio("", "service", "create", catalogService.Meta.Name, service)
			Expect(err).ToNot(HaveOccurred(), out)
		})

		AfterEach(func() {
			By("delete it")
			deleteServiceFromNamespace(namespace, service)
			env.DeleteNamespace(namespace)
		})

		It("shows a service", func() {
			By("show it")
			Eventually(func() string {
				out, err := env.Epinio("", "service", "show", service)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Showing Service"))

				return out
			}, "2m", "5s").Should(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Name", service),
					WithRow("Catalog Service", catalogService.Meta.Name),
					WithRow("Version", catalogService.AppVersion),
					WithRow("Status", "deployed"),
					WithRow("Internal Routes", fmt.Sprintf(`.*\.%s\.svc\.cluster\.local`, namespace)),
				),
			)
		})

		Context("command completion", func() {
			It("matches empty prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "show", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(service))
			})

			It("does not match unknown prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "show", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "service", "show", service, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(service))
			})
		})
	})

	Describe("List", func() {
		var namespace, service string

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			service = catalog.NewServiceName()
		})

		AfterEach(func() {
			By("delete it")
			deleteServiceFromNamespace(namespace, service)
			env.DeleteNamespace(namespace)
		})

		It("list a service", func() {
			By("create it")
			out, err := env.Epinio("", "service", "create", catalogService.Meta.Name, service)
			Expect(err).ToNot(HaveOccurred(), out)

			By("show it")
			out, err = env.Epinio("", "service", "list")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(ContainSubstring("Listing Services"))
			Expect(out).To(ContainSubstring("Namespace: %s", namespace))

			Expect(out).To(
				HaveATable(
					WithHeaders("NAME", "CREATED", "CATALOG SERVICE", "VERSION", "STATUS", "APPLICATIONS"),
					WithRow(service, WithDate(), catalogService.Meta.Name, catalogService.AppVersion, "(not-ready|deployed)", ""),
				),
			)

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "list")
				return out
			}, "2m", "5s").Should(
				HaveATable(
					WithHeaders("NAME", "CREATED", "CATALOG SERVICE", "VERSION", "STATUS", "APPLICATIONS"),
					WithRow(service, WithDate(), catalogService.Meta.Name, catalogService.AppVersion, "deployed", ""),
				),
			)

			By(fmt.Sprintf("%s/%s up", namespace, service))
		})
	})

	Describe("ListAll", func() {
		var namespace1, namespace2 string
		var service1, service2 string
		var tmpSettingsPath string
		var user1, password1 string
		var user2, password2 string

		updateSettings := func(user, password, namespace string) {
			settings, err := settings.LoadFrom(tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred(), settings)

			settings.User = user
			settings.Password = password
			settings.Namespace = namespace
			err = settings.Save()
			Expect(err).ToNot(HaveOccurred())
		}

		BeforeEach(func() {
			namespace1 = catalog.NewNamespaceName()
			namespace2 = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace1)
			env.SetupAndTargetNamespace(namespace2)

			service1 = catalog.NewServiceName()
			service2 = catalog.NewServiceName()

			// create temp settings that we can use to switch users
			tmpSettingsPath = catalog.NewTmpName("tmpEpinio") + `.yaml`
			out, err := env.Epinio("", "settings", "update-ca", "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred(), out)

			// create users with permissions in different namespaces
			user1, password1 = env.CreateEpinioUser("user", []string{namespace1})
			user2, password2 = env.CreateEpinioUser("user", []string{namespace2})
		})

		AfterEach(func() {
			By("delete it")
			deleteServiceFromNamespace(namespace1, service1)
			deleteServiceFromNamespace(namespace2, service2)

			env.DeleteNamespace(namespace1)
			env.DeleteNamespace(namespace2)

			// Remove transient settings
			out, err := proc.Run("", false, "rm", "-f", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred(), out)

			env.DeleteEpinioUser(user1)
			env.DeleteEpinioUser(user2)
		})

		It("list all services", func() {
			By("create them in different namespaces")
			// create service1 in namespace1
			env.TargetNamespace(namespace1)
			out, err := env.Epinio("", "service", "create", catalogService.Meta.Name, service1)
			Expect(err).ToNot(HaveOccurred(), out)

			// create service2 in namespace2
			env.TargetNamespace(namespace2)
			out, err = env.Epinio("", "service", "create", catalogService.Meta.Name, service2)
			Expect(err).ToNot(HaveOccurred(), out)

			// show all the services (we are admin, good to go)
			By("show it")
			out, err = env.Epinio("", "service", "list", "--all")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(ContainSubstring("Listing all Services"))

			Expect(out).To(
				HaveATable(
					WithHeaders("NAMESPACE", "NAME", "CREATED", "CATALOG SERVICE", "VERSION", "STATUS", "APPLICATION"),
					WithRow(namespace1, service1, WithDate(), catalogService.Meta.Name, catalogService.AppVersion, "(not-ready|deployed)", ""),
					WithRow(namespace2, service2, WithDate(), catalogService.Meta.Name, catalogService.AppVersion, "(not-ready|deployed)", ""),
				),
			)

			By("wait for deployment of " + service1)
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "list", "--all")
				return out
			}, "2m", "5s").Should(
				HaveATable(
					WithHeaders("NAMESPACE", "NAME", "CREATED", "CATALOG SERVICE", "VERSION", "STATUS", "APPLICATION"),
					WithRow(namespace1, service1, WithDate(), catalogService.Meta.Name, catalogService.AppVersion, "deployed", ""),
				),
			)

			By(fmt.Sprintf("%s/%s up", namespace1, service1))

			By("wait for deployment of " + service2)
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "list", "--all")
				return out
			}, "2m", "5s").Should(
				HaveATable(
					WithHeaders("NAMESPACE", "NAME", "CREATED", "CATALOG SERVICE", "VERSION", "STATUS", "APPLICATION"),
					WithRow(namespace2, service2, WithDate(), catalogService.Meta.Name, catalogService.AppVersion, "deployed", ""),
				),
			)

			By(fmt.Sprintf("%s/%s up", namespace2, service2))
		})

		It("list only the services in the user namespace", func() {
			By("create them in different namespaces")

			// impersonate user1 and target namespace1
			updateSettings(user1, password1, namespace1)

			// create service1 in namespace1
			out, err := env.Epinio("", "service", "create", catalogService.Meta.Name, service1, "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred(), out, tmpSettingsPath)

			// impersonate user2
			updateSettings(user2, password2, namespace2)

			// create service2 in namespace2
			env.TargetNamespace(namespace2)
			out, err = env.Epinio("", "service", "create", catalogService.Meta.Name, service2, "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred(), out)

			// show only owned namespaces (we are user2, only namespace2)
			By("show it")
			out, err = env.Epinio("", "service", "list", "--all", "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(ContainSubstring("Listing all Services"))

			Expect(out).NotTo(
				HaveATable(
					WithHeaders("NAMESPACE", "NAME", "CREATED", "CATALOG SERVICE", "VERSION", "STATUS", "APPLICATION"),
					WithRow(namespace1, service1, WithDate(), catalogService.Meta.Name, catalogService.AppVersion, "(not-ready|deployed)", ""),
				),
			)
			Expect(out).To(
				HaveATable(
					WithHeaders("NAMESPACE", "NAME", "CREATED", "CATALOG SERVICE", "VERSION", "STATUS", "APPLICATION"),
					WithRow(namespace2, service2, WithDate(), catalogService.Meta.Name, catalogService.AppVersion, "(not-ready|deployed)", ""),
				),
			)

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "list", "--all", "--settings-file", tmpSettingsPath)
				return out
			}, "2m", "5s").Should(
				HaveATable(
					WithHeaders("NAMESPACE", "NAME", "CREATED", "CATALOG SERVICE", "VERSION", "STATUS", "APPLICATION"),
					WithRow(namespace2, service2, WithDate(), catalogService.Meta.Name, catalogService.AppVersion, "deployed", ""),
				),
			)

			By(fmt.Sprintf("%s/%s up", namespace2, service2))
		})
	})

	Describe("Create", func() {
		var namespace, service string

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			service = catalog.NewServiceName()
		})

		AfterEach(func() {
			By("delete it")
			out, err := env.Epinio("", "service", "delete", service)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Services Removed"))

			Eventually(func() string {
				out, _ := env.Epinio("", "service", "delete", service)
				return out
			}, "1m", "5s").Should(ContainSubstring("service '%s' does not exist", service))

			env.DeleteNamespace(namespace)
		})

		It("creates a service", func() {
			By("create it")
			out, err := env.Epinio("", "service", "create", catalogService.Meta.Name, service)
			Expect(err).ToNot(HaveOccurred(), out)

			By("show it")
			out, err = env.Epinio("", "service", "show", service)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Name", service),
					WithRow("Created", WithDate()),
					WithRow("Catalog Service", catalogService.Meta.Name),
					WithRow("Status", "(not-ready|deployed)"),
					WithRow("Internal Routes", fmt.Sprintf(`.*\.%s\.svc\.cluster\.local`, namespace)),
				),
			)

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "show", service)
				return out
			}, "2m", "5s").Should(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Status", "deployed"),
				),
			)

			By(fmt.Sprintf("%s/%s up", namespace, service))
		})

		Context("command completion", func() {
			// A service is not really required, except the outer AfterEach expects it.
			BeforeEach(func() {
				out, err := env.Epinio("", "service", "create", catalogService.Meta.Name, service)
				Expect(err).ToNot(HaveOccurred(), out)
			})
			// Outer AfterEach removes it

			It("matches empty prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "create", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(catalogService.Meta.Name))
			})

			It("does not match unknown prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "create", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "service", "create", catalogService.Meta.Name, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(catalogService.Meta.Name))
			})
		})
	})

	Describe("Delete", func() {
		var namespace, service string

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			service = catalog.NewServiceName()

			By("create it")
			out, err := env.Epinio("", "service", "create", catalogService.Meta.Name, service)
			Expect(err).ToNot(HaveOccurred(), out)

			By("show it")
			out, err = env.Epinio("", "service", "show", service)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Name", service),
					WithRow("Created", WithDate()),
					WithRow("Catalog Service", catalogService.Meta.Name),
					WithRow("Status", "(not-ready|deployed)"),
					WithRow("Internal Routes", fmt.Sprintf(`.*\.%s\.svc\.cluster\.local`, namespace)),
				),
			)

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "show", service)
				return out
			}, "2m", "5s").Should(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Status", "deployed"),
				),
			)

			By(fmt.Sprintf("%s/%s up", namespace, service))
		})

		AfterEach(func() {
			env.DeleteNamespace(namespace)
		})

		It("deletes a service", func() {
			out, err := env.Epinio("", "service", "delete", service)
			Expect(err).ToNot(HaveOccurred(), out)

			Eventually(func() string {
				out, _ := env.Epinio("", "service", "delete", service)
				return out
			}, "1m", "5s").Should(ContainSubstring("service '%s' does not exist", service))
		})

		Context("bulk deletion", func() {
			var service2 string

			BeforeEach(func() {
				service2 = catalog.NewServiceName()
				env.MakeServiceInstance(service2, catalogService.Meta.Name)
			})

			It("deletes multiple services", func() {
				out, err := env.Epinio("", "service", "delete", service, service2)
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					out, _ := env.Epinio("", "service", "show", service)
					return out
				}, "1m", "5s").Should(ContainSubstring("service '%s' does not exist", service))

				Eventually(func() string {
					out, _ := env.Epinio("", "service", "show", service2)
					return out
				}, "1m", "5s").Should(ContainSubstring("service '%s' does not exist", service2))
			})

			It("does match for more than one service", func() {
				out, err := env.Epinio("", "__complete", "service", "delete", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(service))
				Expect(out).To(ContainSubstring(service2))
			})

			It("does match for more than one service but only the remaining one", func() {
				out, err := env.Epinio("", "__complete", "service", "delete", service, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(service))
				Expect(out).To(ContainSubstring(service2))

				out, err = env.Epinio("", "__complete", "service", "delete", service2, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(service))
				Expect(out).ToNot(ContainSubstring(service2))
			})
		})

		When("bound to an app", func() {
			var namespace, service, app, containerImageURL, chart string

			BeforeEach(func() {
				containerImageURL = "splatform/sample-app"

				namespace = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespace)

				service = catalog.NewServiceName()
				chart = names.ServiceReleaseName(service)

				By("create it")
				out, err := env.Epinio("", "service", "create", "mysql-dev", service)
				Expect(err).ToNot(HaveOccurred(), out)

				By("create app")
				app = catalog.NewAppName()
				env.MakeContainerImageApp(app, 1, containerImageURL)

				By("wait for deployment")
				Eventually(func() string {
					out, _ := env.Epinio("", "service", "show", service)
					return out
				}, "2m", "5s").Should(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Status", "deployed"),
					),
				)

				By("bind it")
				out, err = env.Epinio("", "service", "bind", service, app)
				Expect(err).ToNot(HaveOccurred(), out)

				By("verify binding")
				appShowOut, err := env.Epinio("", "app", "show", app)
				Expect(err).ToNot(HaveOccurred())

				Expect(appShowOut).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Bound Configurations", chart+".*"),
					),
				)
			})

			AfterEach(func() {
				env.DeleteNamespace(namespace)
			})

			It("fails to delete a bound service", func() {
				out, err := env.Epinio("", "service", "delete", service)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Unable to delete service. It is still used by"))
				Expect(out).To(ContainSubstring(app))

				// Enable deletion by getting rid of the binding first.

				By("delete app")
				env.DeleteApp(app)

				By("delete it")
				out, err = env.Epinio("", "service", "delete", service)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Services Removed"))

				Eventually(func() string {
					out, _ := env.Epinio("", "service", "delete", service)
					return out
				}, "1m", "5s").Should(ContainSubstring("service '%s' does not exist", service))

			})

			It("unbinds and deletes a bound service when forced", func() {
				out, err := env.Epinio("", "service", "delete", "--unbind", service)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Services Removed"))

				Eventually(func() string {
					out, _ := env.Epinio("", "service", "delete", service)
					return out
				}, "1m", "5s").Should(ContainSubstring("service '%s' does not exist", service))
			})
		})

		Context("command completion", func() {
			It("matches empty prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "delete", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(service))
			})

			It("does not match unknown prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "delete", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does match for more than one argument", func() {
				out, err := env.Epinio("", "__complete", "service", "delete", "fake", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(service))
			})
		})
	})

	Describe("Bind", func() {
		var namespace, service, app, containerImageURL, chart string

		BeforeEach(func() {
			containerImageURL = "splatform/sample-app"

			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			service = catalog.NewServiceName()
			chart = names.ServiceReleaseName(service)

			By("create it")
			out, err := env.Epinio("", "service", "create", "mysql-dev", service)
			Expect(err).ToNot(HaveOccurred(), out)

			By("create app")
			app = catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "show", service)
				return out
			}, "2m", "5s").Should(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Status", "deployed"),
				),
			)
		})

		AfterEach(func() {
			out, err := env.Epinio("", "service", "unbind", service, app)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).ToNot(ContainSubstring("Available Commands:")) // Command should exist

			By("verify unbinding")
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			Expect(appShowOut).ToNot(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Bound Configurations", chart),
				),
			)

			By("delete it")
			out, err = env.Epinio("", "service", "delete", service)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Services Removed"))

			Eventually(func() string {
				out, _ := env.Epinio("", "service", "delete", service)
				return out
			}, "1m", "5s").Should(ContainSubstring("service '%s' does not exist", service))

			env.DeleteNamespace(namespace)
		})

		It("binds the service", func() {
			By("bind it")
			out, err := env.Epinio("", "service", "bind", service, app)
			Expect(err).ToNot(HaveOccurred(), out)

			By("verify binding /app")
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			Expect(appShowOut).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Bound Configurations", chart+".*"),
				),
			)

			By("verify binding /show")
			out, err = env.Epinio("", "service", "show", service)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Used-By", app),
				),
			)

			By("verify binding /list")
			out, err = env.Epinio("", "service", "list")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(
				HaveATable(
					WithHeaders("NAME", "CREATED", "CATALOG SERVICE", "VERSION", "STATUS", "APPLICATIONS"),
					WithRow(service, WithDate(), "mysql-dev", mysqlVersion, "(not-ready|deployed)", app),
				),
			)

			By("verify binding /list-all")
			out, err = env.Epinio("", "service", "list", "--all")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(
				HaveATable(
					WithHeaders("NAMESPACE", "NAME", "CREATED", "CATALOG SERVICE", "VERSION", "STATUS", "APPLICATION"),
					WithRow(namespace, service, WithDate(), "mysql-dev", mysqlVersion, "(not-ready|deployed)", app),
				),
			)
		})

		Context("command completion", func() {
			It("matches empty service prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "bind", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(service))
			})

			It("matches empty app prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "bind", service, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(app))
			})

			It("does not match unknown service prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "bind", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match unknown app prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "bind", service, "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "service", "bind", service, app, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(app))
				Expect(out).ToNot(ContainSubstring(service))
			})
		})
	})

	Describe("Unbind", func() {
		var namespace, service, app, containerImageURL, chart string

		BeforeEach(func() {
			containerImageURL = "splatform/sample-app"

			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			service = catalog.NewServiceName()
			chart = names.ServiceReleaseName(service)

			By("create it")
			out, err := env.Epinio("", "service", "create", "mysql-dev", service)
			Expect(err).ToNot(HaveOccurred(), out)

			By("create app")
			app = catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "show", service)
				return out
			}, "2m", "5s").Should(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Status", "deployed"),
				),
			)

			By("bind it")
			out, err = env.Epinio("", "service", "bind", service, app)
			Expect(err).ToNot(HaveOccurred(), out)

			By("verify binding")
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			Expect(appShowOut).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Bound Configurations", chart+".*"),
				),
			)
		})

		AfterEach(func() {
			By("delete it")
			out, err := env.Epinio("", "service", "delete", service)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Services Removed"))

			Eventually(func() string {
				out, _ := env.Epinio("", "service", "delete", service)
				return out
			}, "1m", "5s").Should(ContainSubstring("service '%s' does not exist", service))

			env.DeleteNamespace(namespace)
		})

		It("unbinds the service", func() {
			out, err := env.Epinio("", "service", "unbind", service, app)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).ToNot(ContainSubstring("Available Commands:")) // Command should exist

			By("verify unbinding")
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			Expect(appShowOut).ToNot(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Bound Configurations", chart+".*"),
				),
			)

			By("verify unbinding /show")
			out, err = env.Epinio("", "service", "show", service)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Used-By", ""),
				),
			)

			By("verify unbinding /list")
			out, err = env.Epinio("", "service", "list")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).ToNot(HaveATable(WithRow(service, WithDate(), "mysql-dev", ".*", app)))
			Expect(out).To(
				HaveATable(
					WithHeaders("NAME", "CREATED", "CATALOG SERVICE", "VERSION", "STATUS", "APPLICATIONS"),
					WithRow(service, WithDate(), "mysql-dev", mysqlVersion, "(not-ready|deployed)", ""),
				),
			)

			By("verify unbinding /list-all")
			out, err = env.Epinio("", "service", "list", "--all")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(
				HaveATable(
					WithHeaders("NAMESPACE", "NAME", "CREATED", "CATALOG SERVICE", "VERSION", "STATUS", "APPLICATION"),
					WithRow(namespace, service, WithDate(), "mysql-dev", mysqlVersion, "(not-ready|deployed)", ""),
				),
			)
		})

		Context("command completion", func() {
			// Needed because the outer BeforeEach does binding, and the tests do not unbind
			AfterEach(func() {
				out, err := env.Epinio("", "service", "unbind", service, app)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("Available Commands:")) // Command should exist
			})

			It("matches empty service prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "unbind", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(service))
			})

			It("matches empty app prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "unbind", service, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(app))
			})

			It("does not match unknown service prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "unbind", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match unknown app prefix", func() {
				out, err := env.Epinio("", "__complete", "service", "unbind", service, "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "service", "unbind", service, app, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(app))
				Expect(out).ToNot(ContainSubstring(service))
			})
		})
	})
})
