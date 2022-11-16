package v1_test

import (
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppCreate Endpoint", LApplication, func() {
	var (
		namespace string
		appName   string
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		appName = catalog.NewAppName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	When("creating a new app", func() {

		AfterEach(func() {
			env.DeleteApp(appName)
		})

		It("creates the app resource", func() {
			response, err := createApplication(appName, namespace, []string{"mytestdomain.org"})
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()

			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
			out, err := proc.Kubectl("get", "apps", "-n", namespace, appName, "-o", "jsonpath={.spec.routes[*]}")
			Expect(err).ToNot(HaveOccurred(), out)
			routes := strings.Split(out, " ")
			Expect(len(routes)).To(Equal(1))
			Expect(routes[0]).To(Equal("mytestdomain.org"))
		})

		It("remembers the chart in the app resource", func() {
			response, err := createApplicationWithChart(appName, namespace, "standard")
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()

			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
			out, err := proc.Kubectl("get", "apps", "-n", namespace, appName, "-o", "jsonpath={.spec.chartname}")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(Equal("standard"))
		})
	})

	When("trying to create a new app with the epinio route", func() {
		It("fails creating the app", func() {
			epinioHost, err := proc.Kubectl("get", "ingress", "--namespace", "epinio", "epinio", "-o", "jsonpath={.spec.rules[*].host}")
			Expect(err).ToNot(HaveOccurred())

			response, err := createApplication(appName, namespace, []string{epinioHost})
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()

			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
		})
	})
})
