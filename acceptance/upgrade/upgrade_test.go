package upgrade_test

import (
	"net/http"
	"os"
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
		domain    string
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()
		domain = os.Getenv("EPINIO_SYSTEM_DOMAIN")
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
		_, err := proc.Run("../..", false, "docker", "build", "-t", "epinio/epinio-server", "-f", "images/Dockerfile", ".")
		Expect(err).NotTo(HaveOccurred())

		// Importing the new image in k3d
		_, err = proc.RunW("k3d", "image", "import", "-c", "epinio-acceptance", "epinio/epinio-server")
		Expect(err).NotTo(HaveOccurred())

		// Upgrade Epinio and use the fresh image by removing the registry value
		_, err = proc.RunW("helm", "upgrade", "epinio",
			"-n", "epinio",
			"../../helm-charts/chart/epinio",
			"--set", "image.epinio.registry=",
			"--set", "image.epinio.tag=latest",
			"--set", "global.domain="+domain,
			"--wait",
		)
		Expect(err).NotTo(HaveOccurred())

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
