package v1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/helpers"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/domain"
	"github.com/epinio/epinio/internal/routes"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppUpdate Endpoint", func() {
	var (
		namespace, containerImageURL string
	)

	BeforeEach(func() {
		containerImageURL = "splatform/sample-app"
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})
	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	When("instances is valid integer", func() {
		It("updates an application with the desired number of instances", func() {
			app := catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
			defer env.DeleteApp(app)

			appObj := appFromAPI(namespace, app)
			Expect(appObj.Workload.Status).To(Equal("1/1"))

			status, _ := updateAppInstances(namespace, app, 3)
			Expect(status).To(Equal(http.StatusOK))

			Eventually(func() string {
				return appFromAPI(namespace, app).Workload.Status
			}, "1m").Should(Equal("3/3"))
		})
	})

	When("instances is invalid", func() {
		It("returns BadRequest when instances is a negative number", func() {
			app := catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
			defer env.DeleteApp(app)
			Expect(appFromAPI(namespace, app).Workload.Status).To(Equal("1/1"))

			status, updateResponseBody := updateAppInstances(namespace, app, -3)
			Expect(status).To(Equal(http.StatusBadRequest))

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
			Expect(appFromAPI(namespace, app).Workload.Status).To(Equal("1/1"))

			status, updateResponseBody := updateAppInstancesNAN(namespace, app)
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
				out, err := helpers.Kubectl("get", "certificates",
					"-n", namespaceName,
					"--selector", "app.kubernetes.io/name="+appName,
					"-o", "jsonpath={.items[*].spec.dnsNames[*]}")
				Expect(err).ToNot(HaveOccurred(), out)
				return len(deleteEmpty(strings.Split(out, " ")))
			}, "20s", "1s").Should(Equal(len(routes)))

			out, err := helpers.Kubectl("get", "certificates",
				"-n", namespaceName,
				"--selector", "app.kubernetes.io/name="+appName,
				"-o", "jsonpath={.items[*].spec.dnsNames[*]}")
			Expect(err).ToNot(HaveOccurred(), out)
			certDomains := deleteEmpty(strings.Split(strings.TrimSpace(out), " "))
			Expect(certDomains).To(ContainElements(routes))
			Expect(len(certDomains)).To(Equal(len(routes)))
		}

		checkIngresses := func(appName, namespaceName string, routesStr ...string) {
			routeObjects := []routes.Route{}
			for _, route := range routesStr {
				routeObjects = append(routeObjects, routes.FromString(route))
			}

			Eventually(func() int {
				out, err := helpers.Kubectl("get", "ingresses",
					"-n", namespaceName,
					"--selector", "app.kubernetes.io/name="+appName,
					"-o", "jsonpath={.items[*].spec.rules[*].host}")
				Expect(err).ToNot(HaveOccurred(), out)
				return len(deleteEmpty(strings.Split(out, " ")))
			}, "20s", "1s").Should(Equal(len(routeObjects)))

			out, err := helpers.Kubectl("get", "ingresses",
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
			Eventually(func() int {
				out, err := helpers.Kubectl("get", "certificates",
					"-n", namespaceName,
					"--selector", "app.kubernetes.io/name="+appName,
					"-o", "jsonpath={.items[*].spec.secretName}")
				Expect(err).ToNot(HaveOccurred(), out)
				certSecrets := deleteEmpty(strings.Split(strings.TrimSpace(out), " "))
				return len(certSecrets)
			}, "20s", "1s").Should(Equal(len(routes)))

			out, err := helpers.Kubectl("get", "certificates",
				"-n", namespaceName,
				"--selector", "app.kubernetes.io/name="+appName,
				"-o", "jsonpath={.items[*].spec.secretName}")
			Expect(err).ToNot(HaveOccurred(), out)
			certSecrets := deleteEmpty(strings.Split(strings.TrimSpace(out), " "))

			Eventually(func() []string {
				out, err = helpers.Kubectl("get", "secrets", "-n", namespaceName, "-o", "jsonpath={.items[*].metadata.name}")
				Expect(err).ToNot(HaveOccurred(), out)
				existingSecrets := deleteEmpty(strings.Split(strings.TrimSpace(out), " "))
				return existingSecrets
			}, "60s", "1s").Should(ContainElements(certSecrets))
		}

		checkRoutesOnApp := func(appName, namespaceName string, routes ...string) {
			out, err := helpers.Kubectl("get", "apps", "-n", namespaceName, appName, "-o", "jsonpath={.spec.routes[*]}")
			Expect(err).ToNot(HaveOccurred(), out)
			appRoutes := deleteEmpty(strings.Split(strings.TrimSpace(out), " "))
			Expect(appRoutes).To(Equal(routes))
		}

		It("synchronizes the ingresses of the application with the new routes list", func() {
			app := catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
			defer env.DeleteApp(app)

			mainDomain, err := domain.MainDomain(context.Background())
			Expect(err).ToNot(HaveOccurred())

			checkRoutesOnApp(app, namespace, fmt.Sprintf("%s.%s", app, mainDomain))
			checkIngresses(app, namespace, fmt.Sprintf("%s.%s", app, mainDomain))
			checkCertificateDNSNames(app, namespace, fmt.Sprintf("%s.%s", app, mainDomain))
			checkSecretsForCerts(app, namespace, fmt.Sprintf("%s.%s", app, mainDomain))

			appObj := appFromAPI(namespace, app)
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
	})
})
