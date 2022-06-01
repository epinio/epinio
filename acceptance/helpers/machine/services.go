package machine

import (
	"fmt"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (m *Machine) MakeServiceInstance(serviceName, catalogService string) {
	By(fmt.Sprintf("MSI %s -> %s", catalogService, serviceName))

	out, err := m.Epinio("", "service", "create", catalogService, serviceName)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check presence and readiness
	Eventually(func() string {
		out, err = m.Epinio("", "service", "show", serviceName)
		Expect(err).ToNot(HaveOccurred(), out)
		Expect(out).To(ContainSubstring(serviceName))

		return out
	}, "2m", "5s").Should(
		HaveATable(
			WithHeaders("KEY", "VALUE"),
			WithRow("Status", "deployed"),
		),
	)

	By("MSI/ok")
}

func (m *Machine) DeleteService(serviceName string) {
	By("deleting a service")
	out, err := m.Epinio("", "service", "delete", serviceName)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	EventuallyWithOffset(1, func() string {
		out, err := m.Epinio("", "service", "list")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "1m").ShouldNot(
		HaveATable(
			WithHeaders("NAME", "CREATED", "CATALOG SERVICE", "STATUS", "APPLICATIONS"),
			WithRow(serviceName, WithDate(), "mysql-dev", "not-ready|deployed", ""),
		),
	)
}
