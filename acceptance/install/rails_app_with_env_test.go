package install_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/testenv"
)

// | Name            | Client Platform       | Server Platform             | Install Test                   | Install Options           | Push Test | Push Options        |
// | --------------- | --------------------- | --------------------------- | ------------------------------ | ------------------------- | --------- | ------------------- |
// |                 | _Actions Runner_      | _Cluster Provider_          | _Operator_                     |                           | _Developer_                     |
// | RailsAppWithEnv | Macos                 | AWS                         | Loadbalancer IP Flag           | Custom Domain             | Rails App | Env Vars            |

// TODO needs DNS Setup
var _ = XDescribe("Install with loadbalancer IP, custom domain and push <RailsAppWithEnv>", func() {
	var (
		flags        []string
		epinioHelper epinio.Epinio
	)
	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		flags = []string{
			"--skip-default-org",
			"--system-domain=test.epinio.io",
			"--loadbalancer-ip=1.1.1.1",
		}
	})

	AfterEach(func() {
		epinioHelper.Uninstall()
	})

	It("installs and passes scenario", func() {
		By("Installing Epinio", func() {
			out, err := epinioHelper.Install(flags...)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Epinio installed."))

			out, err = testenv.PatchEpinio()
			Expect(err).ToNot(HaveOccurred(), out)
		})

		// Now create the default org which we skipped because
		// it would fail before patching.
		testenv.EnsureDefaultWorkspace(testenv.EpinioBinaryPath())

		By("Pushing an app", func() {
			// with env vars
		})
	})
})
