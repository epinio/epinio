package acceptance_test

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Services", func() {
	Describe("delete services", func() {
		var namespace, service string

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			service = catalog.NewServiceName()

			out, err := env.Epinio("", "service", "create", "mysql-dev", service)
			Expect(err).ToNot(HaveOccurred(), out)

			out, err = env.Epinio("", "service", "show", service)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(fmt.Sprintf("Name.*\\|.*%s", service)))
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
	})
})
