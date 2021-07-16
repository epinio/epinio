package machine

import (
	"fmt"

	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/helpers"
)

func (m *Machine) MakeCustomService(serviceName string) {
	out, err := m.Epinio(fmt.Sprintf("service create-custom %s username epinio-user", serviceName), "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check presence

	out, err = m.Epinio("service list", "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp(serviceName))
}

func (m *Machine) MakeCatalogService(serviceName string, dataJSON ...string) {
	dataStr := ""
	if len(dataJSON) > 0 {
		dataStr = fmt.Sprintf("--data '%s'", dataJSON[0])
	}
	out, err := m.Epinio(fmt.Sprintf("service create %s mariadb 10-3-22 %s", serviceName, dataStr), "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// Look for the messaging indicating that the command waited
	ExpectWithOffset(1, out).To(MatchRegexp("Provisioning"))
	ExpectWithOffset(1, out).To(MatchRegexp("Service Provisioned"))

	// Check presence

	out, err = m.Epinio("service list", "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp(serviceName))
}

func (m *Machine) MakeCatalogServiceDontWait(serviceName string, dataJSON ...string) {
	dataStr := ""
	if len(dataJSON) > 0 {
		dataStr = fmt.Sprintf("--data '%s'", dataJSON[0])
	}
	out, err := m.Epinio(fmt.Sprintf("service create --dont-wait %s mariadb 10-3-22 %s", serviceName, dataStr), "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// Look for indicator that command did not wait
	ExpectWithOffset(1, out).To(MatchRegexp("to watch when it is provisioned"))

	// Check presence

	out, err = m.Epinio("service list", "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp(serviceName))

	// And explicitly wait for it being provisioned

	EventuallyWithOffset(1, func() string {
		out, err = m.Epinio("service show "+serviceName, "")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "5m").Should(MatchRegexp(`Status .*\|.* Provisioned`))
}

func (m *Machine) BindAppService(appName, serviceName, org string) {
	out, err := m.Epinio(fmt.Sprintf("service bind %s %s", serviceName, appName), "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check deep into the kube structures
	m.VerifyAppServiceBound(appName, serviceName, org, 2)
}

func (m *Machine) VerifyAppServiceBound(appName, serviceName, org string, offset int) {
	out, err := helpers.Kubectl(fmt.Sprintf("get deployment -n %s %s -o=jsonpath='{.spec.template.spec.volumes}'", org, appName))
	ExpectWithOffset(offset, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(offset, out).To(MatchRegexp(serviceName))

	out, err = helpers.Kubectl(fmt.Sprintf("get deployment -n %s %s -o=jsonpath='{.spec.template.spec.containers[0].volumeMounts}'", org, appName))
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp("/services/" + serviceName))
}

func (m *Machine) DeleteService(serviceName string) {
	m.DeleteServiceWithUnbind(serviceName, false)
}

func (m *Machine) DeleteServiceUnbind(serviceName string) {
	m.DeleteServiceWithUnbind(serviceName, true)
}

func (m *Machine) DeleteServiceWithUnbind(serviceName string, unbind bool) {
	unbindFlag := ""
	if unbind {
		unbindFlag = "--unbind"
	}
	out, err := m.Epinio(fmt.Sprintf("service delete %s %s", serviceName, unbindFlag), "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check non-presence
	EventuallyWithOffset(1, func() string {
		out, err = m.Epinio("service list", "")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "10m").ShouldNot(MatchRegexp(serviceName))
}

func (m *Machine) CleanupService(serviceName string) {
	out, err := m.Epinio("service delete "+serviceName, "")

	if err != nil {
		fmt.Printf("deleting service failed : %s\n%s", err.Error(), out)
	}
}

func (m *Machine) UnbindAppService(appName, serviceName, org string) {
	out, err := m.Epinio(fmt.Sprintf("service unbind %s %s", serviceName, appName), "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And deep check in kube structures for non-presence
	m.VerifyAppServiceNotbound(appName, serviceName, org, 2)
}

func (m *Machine) VerifyAppServiceNotbound(appName, serviceName, org string, offset int) {
	out, err := helpers.Kubectl(fmt.Sprintf("get deployment -n %s %s -o=jsonpath='{.spec.template.spec.volumes}'", org, appName))
	ExpectWithOffset(offset, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(offset, out).ToNot(MatchRegexp(serviceName))

	out, err = helpers.Kubectl(fmt.Sprintf("get deployment -n %s %s -o=jsonpath='{.spec.template.spec.containers[0].volumeMounts}'", org, appName))
	ExpectWithOffset(offset, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(offset, out).ToNot(MatchRegexp("/services/" + serviceName))
}
