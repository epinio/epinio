package machine

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	. "github.com/onsi/gomega"
)

func (m *Machine) MakeConfiguration(configurationName string) {
	out, err := m.Epinio("", "configuration", "create", configurationName, "username", "epinio-user")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check presence

	out, err = m.Epinio("", "configuration", "list")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp(configurationName))
}

func (m *Machine) BindAppConfiguration(appName, configurationName, namespace string) {
	out, err := m.Epinio("", "configuration", "bind", configurationName, appName)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check deep into the kube structures
	m.VerifyAppConfigurationBound(appName, configurationName, namespace, 2)
}

func (m *Machine) VerifyAppConfigurationBound(appName, configurationName, namespace string, offset int) {
	out, err := proc.Kubectl("get", "deployment",
		"--namespace", namespace, appName,
		"-o", "jsonpath={.spec.template.spec.volumes}")
	ExpectWithOffset(offset, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(offset, out).To(MatchRegexp(configurationName))

	out, err = proc.Kubectl("get", "deployment",
		"--namespace", namespace, appName,
		"-o", "jsonpath={.spec.template.spec.containers[0].volumeMounts}")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp("/configurations/" + configurationName))
}

func (m *Machine) DeleteConfiguration(configurationName string) {
	m.DeleteConfigurationWithUnbind(configurationName, false)
}

func (m *Machine) DeleteConfigurationUnbind(configurationName string) {
	m.DeleteConfigurationWithUnbind(configurationName, true)
}

func (m *Machine) DeleteConfigurationWithUnbind(configurationName string, unbind bool) {
	var err error
	var out string
	if unbind {
		out, err = m.Epinio("", "configuration", "delete", configurationName, "--unbind")
	} else {
		out, err = m.Epinio("", "configuration", "delete", configurationName)
	}
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check non-presence
	EventuallyWithOffset(1, func() string {
		out, err = m.Epinio("", "configuration", "list")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "10m").ShouldNot(MatchRegexp(configurationName))
}

func (m *Machine) CleanupConfiguration(configurationName string) {
	out, err := m.Epinio("", "configuration", "delete", configurationName)

	if err != nil {
		fmt.Printf("deleting configuration failed : %s\n%s", err.Error(), out)
	}
}

func (m *Machine) UnbindAppConfiguration(appName, configurationName, namespace string) {
	out, err := m.Epinio("", "configuration", "unbind", configurationName, appName)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And deep check in kube structures for non-presence
	m.VerifyAppConfigurationNotbound(appName, configurationName, namespace, 2)
}

func (m *Machine) VerifyAppConfigurationNotbound(appName, configurationName, namespace string, offset int) {
	out, err := proc.Kubectl("get", "deployment",
		"--namespace", namespace, appName,
		"-o", "jsonpath={.spec.template.spec.volumes}")
	ExpectWithOffset(offset, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(offset, out).ToNot(MatchRegexp(configurationName))

	out, err = proc.Kubectl("get", "deployment",
		"--namespace", namespace, appName,
		"-o", "jsonpath={.spec.template.spec.containers[0].volumeMounts}")
	ExpectWithOffset(offset, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(offset, out).ToNot(MatchRegexp("/configurations/" + configurationName))
}
