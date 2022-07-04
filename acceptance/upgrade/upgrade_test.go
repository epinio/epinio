package upgrade_test

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Epinio upgrade with running app", func() {
	var (
		namespace string
		appName   string
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()
	})

	AfterEach(func() {
		env.DeleteApp(appName)
		env.DeleteNamespace(namespace)
	})

	It("can upgrade epinio", func() {
		// Deploy a simple application before upgrading Epinio
		out := env.MakeGolangApp(appName, 1, true)
		routeRegexp := regexp.MustCompile(`https:\/\/.*omg.howdoi.website`)
		route := string(routeRegexp.Find([]byte(out)))

		// Check that the app is reachable
		Eventually(func() int {
			resp, err := env.Curl("GET", route, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			return resp.StatusCode
		}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

		// Build image with latest epinio-server binary
		out, err := proc.Run("../..", false, "docker", "build", "-t", "epinio/epinio-server", "-f", "images/Dockerfile", ".")
		Expect(err).NotTo(HaveOccurred(), out)

		// Importing the new image in k3d
		out, err = proc.RunW("k3d", "image", "import", "-c", "epinio-acceptance", "epinio/epinio-server")
		Expect(err).NotTo(HaveOccurred(), out)

		// Upgrade Epinio and use the fresh image by removing the registry value
		out, err = proc.RunW("helm", "upgrade", "--reuse-values", "epinio",
			"-n", "epinio",
			"../../helm-charts/chart/epinio",
			"--set", "image.epinio.registry=",
			"--set", "image.epinio.tag=latest",
			"--wait",
		)
		Expect(err).NotTo(HaveOccurred(), out)

		// Check that the app is still reachable
		Eventually(func() int {
			resp, err := env.Curl("GET", route, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			return resp.StatusCode
		}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

		// We can think about adding more checks later like application with
		// environment vars or configurations

	})
})
