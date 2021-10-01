package machine

import (
	"fmt"

	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/helpers"
)

func (m *Machine) MakeCustomService(serviceName string) {
	out, err := m.Epinio("", "service", "create-custom", serviceName, "username", "epinio-user")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check presence

	out, err = m.Epinio("", "service", "list")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp(serviceName))
}

func (m *Machine) BindAppService(appName, serviceName, org string) {
	out, err := m.Epinio("", "service", "bind", serviceName, appName)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check deep into the kube structures
	m.VerifyAppServiceBound(appName, serviceName, org, 2)
}

func (m *Machine) VerifyAppServiceBound(appName, serviceName, org string, offset int) {
	out, err := helpers.Kubectl("get", "deployment",
		"--namespace", org, appName,
		"-o", "jsonpath={.spec.template.spec.volumes}")
	ExpectWithOffset(offset, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(offset, out).To(MatchRegexp(serviceName))

	out, err = helpers.Kubectl("get", "deployment",
		"--namespace", org, appName,
		"-o", "jsonpath={.spec.template.spec.containers[0].volumeMounts}")
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
	var err error
	var out string
	if unbind {
		out, err = m.Epinio("", "service", "delete", serviceName, "--unbind")
	} else {
		out, err = m.Epinio("", "service", "delete", serviceName)
	}
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check non-presence
	EventuallyWithOffset(1, func() string {
		out, err = m.Epinio("", "service", "list")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "10m").ShouldNot(MatchRegexp(serviceName))
}

func (m *Machine) CleanupService(serviceName string) {
	out, err := m.Epinio("", "service", "delete", serviceName)

	if err != nil {
		fmt.Printf("deleting service failed : %s\n%s", err.Error(), out)
	}
}

func (m *Machine) UnbindAppService(appName, serviceName, org string) {
	out, err := m.Epinio("", "service", "unbind", serviceName, appName)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And deep check in kube structures for non-presence
	m.VerifyAppServiceNotbound(appName, serviceName, org, 2)
}

func (m *Machine) VerifyAppServiceNotbound(appName, serviceName, org string, offset int) {
	out, err := helpers.Kubectl("get", "deployment",
		"--namespace", org, appName,
		"-o", "jsonpath={.spec.template.spec.volumes}")
	ExpectWithOffset(offset, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(offset, out).ToNot(MatchRegexp(serviceName))

	out, err = helpers.Kubectl("get", "deployment",
		"--namespace", org, appName,
		"-o", "jsonpath={.spec.template.spec.containers[0].volumeMounts}")
	ExpectWithOffset(offset, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(offset, out).ToNot(MatchRegexp("/services/" + serviceName))
}
