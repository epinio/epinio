package acceptance_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Carrier maintenance operations", func() {
	Describe("info", func() {
		It("prints information about the Carrier components and platform", func() {
			info, err := Carrier("info", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(info).To(MatchRegexp("Platform: k3s"))
			Expect(info).To(MatchRegexp("Kubernetes Version: v1.20"))
			Expect(info).To(MatchRegexp("Gitea Version: 1.13"))
		})
	})

	Describe("uninstall", func() {
		AfterEach(func() {
			// TODO: This could fail because when we uninstall we don't wait for things
			// to be removed.
			installCarrier()
			// Allow things to settle. Shouldn't be needed after we fix this:
			// https://github.com/SUSE/carrier/issues/108
			time.Sleep(3 * time.Minute)
		})

		It("uninstalls Carrier", func() {
			out, err := Carrier("uninstall", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(MatchRegexp("Carrier uninstalled"))
		})
	})
})
