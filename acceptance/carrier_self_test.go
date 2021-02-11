package acceptance_test

import (
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
})
