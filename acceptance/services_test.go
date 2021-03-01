package acceptance_test

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/suse/carrier/helpers"
)

var _ = Describe("Services", func() {
	var org = "apps-org"
	BeforeEach(func() {
		out, err := Carrier("create-org "+org, "")
		Expect(err).ToNot(HaveOccurred(), out)
		out, err = Carrier("target "+org, "")
		Expect(err).ToNot(HaveOccurred(), out)
	})
	Describe("create-custom-service", func() {
		var serviceName string
		BeforeEach(func() {
			serviceName = "service-" + strconv.Itoa(int(time.Now().Nanosecond()))
		})

		It("creates a custom service", func() {
			out, err := Carrier(fmt.Sprintf("create-custom-service %s username carrier-user", serviceName), "")
			Expect(err).ToNot(HaveOccurred(), out)
			out, err = Carrier("services", "")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(serviceName))
		})
	})

	Describe("delete service", func() {
		var serviceName string
		BeforeEach(func() {
			serviceName = "service-" + strconv.Itoa(int(time.Now().Nanosecond()))
			out, err := Carrier(fmt.Sprintf("create-custom-service %s username carrier-user", serviceName), "")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("creates a custom service", func() {
			out, err := Carrier("delete-service "+serviceName, "")
			Expect(err).ToNot(HaveOccurred(), out)
			out, err = Carrier("services", "")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).ToNot(MatchRegexp(serviceName))
		})

		PIt("doesn't delete a bound service", func() {
		})
	})

	Describe("bind-service", func() {
		var serviceName, appName string
		BeforeEach(func() {
			serviceName = "service-" + strconv.Itoa(int(time.Now().Nanosecond()))
			appName = "apps-" + strconv.Itoa(int(time.Now().Nanosecond()))

			out, err := Carrier(fmt.Sprintf("create-custom-service %s username carrier-user", serviceName), "")
			Expect(err).ToNot(HaveOccurred(), out)

			currentDir, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())
			appDir := path.Join(currentDir, "../sample-app")
			out, err = Carrier(fmt.Sprintf("push %s --verbosity 1", appName), appDir)
			Expect(err).ToNot(HaveOccurred(), out)
		})

		AfterEach(func() {
			out, err := Carrier("delete "+appName, "")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("binds a service to the application deployment", func() {
			out, err := Carrier(fmt.Sprintf("bind-service %s %s", serviceName, appName), "")
			Expect(err).ToNot(HaveOccurred(), out)
			out, err = helpers.Kubectl(fmt.Sprintf("get deployment -n carrier-workloads %s.%s -o=jsonpath='{.spec.template.spec.volumes}'", org, appName))
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(serviceName))

			out, err = helpers.Kubectl(fmt.Sprintf("get deployment -n carrier-workloads %s.%s -o=jsonpath='{.spec.template.spec.containers[0].volumeMounts}'", org, appName))
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("/services/" + serviceName))
		})
	})
})
