package machine

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (m *Machine) MakeServiceInstance(serviceName, catalogService string) {
	out, err := m.Epinio("", "service", "create", catalogService, serviceName)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check presence and readiness
	Eventually(func() string {
		out, err = m.Epinio("", "service", "show", serviceName)
		Expect(err).ToNot(HaveOccurred(), out)
		Expect(out).To(MatchRegexp(serviceName))

		return out
	}, "2m", "5s").Should(MatchRegexp("Status.*|.*deployed"))
}

func (m *Machine) DeleteService(serviceName string) {
	By("deleting a service")
	out, err := m.Epinio("", "service", "delete", serviceName)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	EventuallyWithOffset(1, func() string {
		out, err := m.Epinio("", "service", "list")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "1m").ShouldNot(MatchRegexp(`.*%s.*`, serviceName))
}
