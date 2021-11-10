package install_test

import (
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/testenv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Install with <InvalidFlags>", func() {
	var (
		flags        []string
		epinioHelper epinio.Epinio
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())
	})

	When("force-kube-internal-registry-tls is used with an external registry", func() {
		BeforeEach(func() {
			flags = []string{
				"--force-kube-internal-registry-tls",
				"--external-registry-url=someregistry.com",
			}
		})
		It("returns an error", func() {
			By("Installing epinio")
			out, err := epinioHelper.Install(flags...)
			Expect(err).To(HaveOccurred())
			Expect(out).To(MatchRegexp("error installing Epinio: force-kube-internal-registry-tls has no effect when an external registry is used"))
		})
	})
})
