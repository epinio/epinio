package machine

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/proc"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (m *Machine) HaveServiceInstance(serviceName string) {
	By(fmt.Sprintf("HSI %s", serviceName))

	// And check presence and readiness
	out, err := m.Epinio("", "service", "show", serviceName)
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).To(ContainSubstring(serviceName))
	Expect(out).To(HaveATable(
		WithHeaders("KEY", "VALUE"),
		WithRow("Status", "deployed"),
	))
	By("HSI/ok")
}

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
	}, "5m", "5s").Should(
		HaveATable(
			WithHeaders("KEY", "VALUE"),
			WithRow("Status", "deployed"),
		),
		func() string {
			outNamespace, _ := m.Epinio(m.nodeTmpDir, "target")
			outPods, _ := proc.Kubectl("get", "pods", "-A")
			outHelm, _ := proc.Run("", false, "helm", "list", "-a", "-A")
			return fmt.Sprintf("%s\nPods:\n%s\nHelm releases:\n%s\n", outNamespace, outPods, outHelm)
		}(),
	)

	By("MSI/ok")
}

func (m *Machine) DeleteService(serviceName string) {
	By("deleting a service: " + serviceName)

	out, err := m.Epinio("", "service", "delete", serviceName)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	Expect(out).To(ContainSubstring("Service Removed"))

	Eventually(func() string {
		out, _ := m.Epinio("", "service", "delete", serviceName)
		return out
	}, "1m", "5s").Should(ContainSubstring("service '%s' does not exist", serviceName))
}
