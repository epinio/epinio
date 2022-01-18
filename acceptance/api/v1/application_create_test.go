package v1_test

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppCreate Endpoint", func() {
	var (
		namespace string
		appName   string
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()
	})

	AfterEach(func() {
		Eventually(func() string {
			out, err := env.Epinio("", "app", "delete", appName)
			if err != nil {
				return out
			}
			return ""
		}, "5m").Should(BeEmpty())

		env.DeleteNamespace(namespace)
	})

	When("creating a new app", func() {
		It("creates the app resource", func() {
			response, err := createApplication(appName, namespace, []string{"mytestdomain.org"})
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()

			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
			out, err := proc.Kubectl("get", "apps", "-n", namespace, appName, "-o", "jsonpath={.spec.routes[*]}")
			Expect(err).ToNot(HaveOccurred(), out)
			routes := strings.Split(out, " ")
			Expect(len(routes)).To(Equal(1))
			Expect(routes[0]).To(Equal("mytestdomain.org"))
		})
	})
})
