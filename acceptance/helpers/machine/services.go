package machine

import (
	"fmt"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (m *Machine) MakeServiceInstance(serviceName, catalogService string) {
	By(fmt.Sprintf("MSI %s -> %s", catalogService, serviceName))

	_ = m.Epinio("", "service", "create", catalogService, serviceName)

	// And check presence and readiness
	Eventually(func() string {
		out := m.Epinio("", "service", "show", serviceName)
		Expect(out).To(ContainSubstring(serviceName))
		return out
	}, "5m", "5s").Should(
		HaveATable(
			WithHeaders("KEY", "VALUE"),
			WithRow("Status", "deployed"),
		),
		func() string {
			outNamespace := m.Epinio(m.nodeTmpDir, "target")
			outPods, _ := proc.Kubectl("get", "pods", "-A")
			outHelm, _ := proc.Run("", false, "helm", "list", "-a", "-A")
			return fmt.Sprintf("%s\nPods:\n%s\nHelm releases:\n%s\n", outNamespace, outPods, outHelm)
		}(),
	)

	By("MSI/ok")
}

func (m *Machine) DeleteService(serviceName string) {
	By("deleting a service")
	_ = m.Epinio("", "service", "delete", serviceName)

	EventuallyWithOffset(1,
		m.Epinio("", "service", "list"),
		"1m",
	).ShouldNot(
		HaveATable(
			WithHeaders("NAME", "CREATED", "CATALOG SERVICE", "STATUS", "APPLICATIONS"),
			WithRow(serviceName, WithDate(), "mysql-dev", "(not-ready|deployed)", ""),
		),
	)
}
