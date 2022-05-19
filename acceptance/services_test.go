package acceptance_test

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/helpers/regex"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("Services", func() {

	Describe("Catalog", func() {
		It("lists the standard catalog", func() {
			out, err := env.Epinio("", "service", "catalog")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Getting catalog"))
			Expect(out).To(MatchRegexp("mysql-dev"))
			Expect(out).To(MatchRegexp("postgresql-dev"))
			Expect(out).To(MatchRegexp("rabbitmq-dev"))
			Expect(out).To(MatchRegexp("redis-dev"))
		})

		It("lists the catalog details", func() {
			out, err := env.Epinio("", "service", "catalog", "redis-dev")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Name.*\\|.*redis-dev"))
			Expect(out).To(MatchRegexp("Short Description.*\\|.*A Redis service"))
			Expect(out).To(MatchRegexp("Bitnami Redis instance"))
			Expect(out).To(MatchRegexp("Version.*\\|.*6.2.7"))
		})

		When("Adding a catalog entry", func() {
			var catalogService models.CatalogService
			var serviceName string

			BeforeEach(func() {
				serviceName = catalog.NewCatalogServiceName()

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
				catalog.DeleteCatalogService(serviceName)
			})

			It("lists the extended catalog", func() {
				out, err := env.Epinio("", "service", "catalog")
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(out).To(MatchRegexp("Getting catalog"))
				Expect(out).To(MatchRegexp(serviceName))
			})

			It("lists the extended catalog details", func() {
				out, err := env.Epinio("", "service", "catalog", serviceName)
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(out).To(MatchRegexp(fmt.Sprintf("Name.*\\|.*%s", serviceName)))
			})
		})
	})

	deleteServiceFromNamespace := func(namespace, service string) {
		env.TargetNamespace(namespace)

		out, err := env.Epinio("", "service", "delete", service)
		Expect(err).ToNot(HaveOccurred(), out)
		Expect(out).To(MatchRegexp("Service Removed"))

		Eventually(func() string {
			out, _ := env.Epinio("", "service", "delete", service)
			return out
		}, "1m", "5s").Should(MatchRegexp("service not found"))

	}

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
			out, err := env.Epinio("", "service", "create", "mysql-dev", service)
			Expect(err).ToNot(HaveOccurred(), out)

			By("show it")
			out, err = env.Epinio("", "service", "list")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Listing Services"))
			Expect(out).To(MatchRegexp("Namespace: " + namespace))

			Expect(out).To(MatchRegexp(regex.TableRow(service, regex.DateRegex, "mysql-dev", "not-ready")))

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "list")
				return out
			}, "2m", "5s").Should(MatchRegexp("deployed"))

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
			out, err := env.Epinio("", "settings", "update", "--settings-file", tmpSettingsPath)
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
			out, err := env.Epinio("", "service", "create", "mysql-dev", service1)
			Expect(err).ToNot(HaveOccurred(), out)

			// create service2 in namespace2
			env.TargetNamespace(namespace2)
			out, err = env.Epinio("", "service", "create", "mysql-dev", service2)
			Expect(err).ToNot(HaveOccurred(), out)

			// show all the services (we are admin, good to go)
			By("show it")
			out, err = env.Epinio("", "service", "list", "--all")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Listing all Services"))

			Expect(out).To(MatchRegexp(regex.TableRow(namespace1, service1, "mysql-dev", "not-ready")))
			Expect(out).To(MatchRegexp(regex.TableRow(namespace2, service2, "mysql-dev", "not-ready")))

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "list", "--all")
				return out
			}, "2m", "5s").Should(MatchRegexp(regex.TableRow(namespace1, service1, "mysql-dev", "deployed")))

			Eventually(func() string {
				out, _ := env.Epinio("", "service", "list", "--all")
				return out
			}, "2m", "5s").Should(MatchRegexp(regex.TableRow(namespace2, service2, "mysql-dev", "deployed")))

			By(fmt.Sprintf("%s/%s up", namespace1, service1))
		})

		It("list only the services in the user namespace", func() {
			By("create them in different namespaces")

			// impersonate user1 and target namespace1
			updateSettings(user1, password1, namespace1)

			// create service1 in namespace1
			out, err := env.Epinio("", "service", "create", "mysql-dev", service1, "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred(), out, tmpSettingsPath)

			// impersonate user2
			updateSettings(user2, password2, namespace2)

			// create service2 in namespace2
			env.TargetNamespace(namespace2)
			out, err = env.Epinio("", "service", "create", "mysql-dev", service2, "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred(), out)

			// show only owned namespaces (we are user2, only namespace2)
			By("show it")
			out, err = env.Epinio("", "service", "list", "--all", "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Listing all Services"))

			Expect(out).NotTo(MatchRegexp(regex.TableRow(namespace1, service1, "mysql-dev", "not-ready")))
			Expect(out).To(MatchRegexp(regex.TableRow(namespace2, service2, "mysql-dev", "not-ready")))

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "list", "--all", "--settings-file", tmpSettingsPath)
				return out
			}, "2m", "5s").Should(MatchRegexp(regex.TableRow(namespace2, service2, "mysql-dev", "deployed")))

			By(fmt.Sprintf("%s/%s up", namespace1, service1))
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
			Expect(out).To(MatchRegexp("Service Removed"))

			Eventually(func() string {
				out, _ := env.Epinio("", "service", "delete", service)
				return out
			}, "1m", "5s").Should(MatchRegexp("service not found"))

			env.DeleteNamespace(namespace)
		})

		It("creates a service", func() {
			By("create it")
			out, err := env.Epinio("", "service", "create", "mysql-dev", service)
			Expect(err).ToNot(HaveOccurred(), out)

			By("show it")
			out, err = env.Epinio("", "service", "show", service)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(fmt.Sprintf("Name.*\\|.*%s", service)))

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "show", service)
				return out
			}, "2m", "5s").Should(MatchRegexp("Status.*\\|.*deployed"))

			By(fmt.Sprintf("%s/%s up", namespace, service))
		})
	})

	Describe("Delete", func() {
		var namespace, service string

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			service = catalog.NewServiceName()

			By("create it")
			out, err := env.Epinio("", "service", "create", "mysql-dev", service)
			Expect(err).ToNot(HaveOccurred(), out)

			By("show it")
			out, err = env.Epinio("", "service", "show", service)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(fmt.Sprintf("Name.*\\|.*%s", service)))

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "show", service)
				return out
			}, "2m", "5s").Should(MatchRegexp("Status.*\\|.*deployed"))

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
			}, "1m", "5s").Should(MatchRegexp("service not found"))
		})

		When("bound to an app", func() {
			var namespace, service, app, containerImageURL, chart string

			BeforeEach(func() {
				containerImageURL = "splatform/sample-app"

				namespace = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespace)

				service = catalog.NewServiceName()
				chart = names.ServiceHelmChartName(service, namespace)

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
				}, "2m", "5s").Should(MatchRegexp("Status.*\\|.*deployed"))

				By("bind it")
				out, err = env.Epinio("", "service", "bind", service, app)
				Expect(err).ToNot(HaveOccurred(), out)

				By("verify binding")
				appShowOut, err := env.Epinio("", "app", "show", app)
				Expect(err).ToNot(HaveOccurred())
				matchString := fmt.Sprintf("Bound Configurations.*%s", chart)
				Expect(appShowOut).To(MatchRegexp(matchString))
			})

			AfterEach(func() {
				env.DeleteNamespace(namespace)
			})

			It("fails to delete a bound service", func() {
				out, err := env.Epinio("", "service", "delete", service)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp("Unable to delete service. It is still used by"))
				Expect(out).To(MatchRegexp(app))

				// Enable deletion by getting rid of the binding first.

				By("delete app")
				env.DeleteApp(app)

				By("delete it")
				out, err = env.Epinio("", "service", "delete", service)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp("Service Removed"))

				Eventually(func() string {
					out, _ := env.Epinio("", "service", "delete", service)
					return out
				}, "1m", "5s").Should(MatchRegexp("service not found"))

			})

			It("unbinds and deletes a bound service when forced", func() {
				out, err := env.Epinio("", "service", "delete", "--unbind", service)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp("Service Removed"))

				Eventually(func() string {
					out, _ := env.Epinio("", "service", "delete", service)
					return out
				}, "1m", "5s").Should(MatchRegexp("service not found"))
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
			chart = names.ServiceHelmChartName(service, namespace)

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
			}, "2m", "5s").Should(MatchRegexp("Status.*\\|.*deployed"))
		})

		AfterEach(func() {
			out, err := env.Epinio("", "service", "unbind", service, app)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).ToNot(MatchRegexp("Available Commands:")) // Command should exist

			By("verify unbinding")
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			matchString := fmt.Sprintf("Bound Configurations.*%s", chart)
			Expect(appShowOut).ToNot(MatchRegexp(matchString))

			By("delete it")
			out, err = env.Epinio("", "service", "delete", service)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Service Removed"))

			Eventually(func() string {
				out, _ := env.Epinio("", "service", "delete", service)
				return out
			}, "1m", "5s").Should(MatchRegexp("service not found"))

			env.DeleteNamespace(namespace)
		})

		It("binds the service", func() {
			By("bind it")
			out, err := env.Epinio("", "service", "bind", service, app)
			Expect(err).ToNot(HaveOccurred(), out)

			By("verify binding")
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			matchString := fmt.Sprintf("Bound Configurations.*%s", chart)
			Expect(appShowOut).To(MatchRegexp(matchString))
		})
	})

	Describe("Unbind", func() {
		var namespace, service, app, containerImageURL, chart string

		BeforeEach(func() {
			containerImageURL = "splatform/sample-app"

			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			service = catalog.NewServiceName()
			chart = names.ServiceHelmChartName(service, namespace)

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
			}, "2m", "5s").Should(MatchRegexp("Status.*\\|.*deployed"))

			By("bind it")
			out, err = env.Epinio("", "service", "bind", service, app)
			Expect(err).ToNot(HaveOccurred(), out)

			By("verify binding")
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			matchString := fmt.Sprintf("Bound Configurations.*%s", chart)
			Expect(appShowOut).To(MatchRegexp(matchString))
		})

		AfterEach(func() {
			By("delete it")
			out, err := env.Epinio("", "service", "delete", service)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Service Removed"))

			Eventually(func() string {
				out, _ := env.Epinio("", "service", "delete", service)
				return out
			}, "1m", "5s").Should(MatchRegexp("service not found"))

			env.DeleteNamespace(namespace)
		})

		It("unbinds the service", func() {
			out, err := env.Epinio("", "service", "unbind", service, app)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).ToNot(MatchRegexp("Available Commands:")) // Command should exist

			By("verify unbinding")
			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			matchString := fmt.Sprintf("Bound Configurations.*%s", chart)
			Expect(appShowOut).ToNot(MatchRegexp(matchString))
		})
	})
})
