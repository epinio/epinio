package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bounds between Apps & Services", func() {
	var org string
	dockerImageURL := "rohitsakala/app1"
	//TODO: change this docker image url

	BeforeEach(func() {
		org = catalog.NewOrgName()
		env.SetupAndTargetOrg(org)
	})
	Describe("Display", func() {
		var appName string
		var serviceName string
		BeforeEach(func() {
			appName = catalog.NewAppName()
			serviceName = catalog.NewServiceName()

			env.MakeDockerImageApp(appName, 1, dockerImageURL)
			env.MakeCustomService(serviceName)
			env.BindAppService(appName, serviceName, org)
		})
		It("shows the bound app for services list, and vice versa", func() {
			out, err := env.Epinio("service list", "")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(serviceName + `.*` + appName))

			// The next check uses `Eventually` because binding the
			// service to the app forces a restart of the app's
			// pod. It takes the system some time to terminate the
			// old pod, and spin up the new, during which `app list`
			// will return inconsistent results about the desired
			// and actual number of instances. We wait for the
			// system to settle back into a normal state.

			Eventually(func() string {
				out, err = env.Epinio("app list", "")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "5m").Should(MatchRegexp(appName + `.*\|.*1\/1.*\|.*` + serviceName))
		})
		AfterEach(func() {
			// Delete app first, as this also unbinds the service
			env.CleanupApp(appName)
			env.CleanupService(serviceName)
		})
	})
})
