package upgrade_test

import (
	"fmt"
	"net/http"
	"os"
	"path"
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

		// Redundant, done by prepare_environment_k3d in flow setup (upgrade.yml)
		// Build image with latest epinio-server binary
		By("Building image ...")
		out, err := proc.Run("../..", false, "docker", "build", "-t", "epinio/epinio-server", "-f", "images/Dockerfile", ".")
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)

		tag := os.Getenv("EPINIO_CURRENT_TAG")
		By("Tag: " + tag)

		// Importing the new image in k3d
		By("Importing image ...")
		out, err = proc.RunW("k3d", "image", "import", "-c", "epinio-acceptance",
			fmt.Sprintf("ghcr.io/epinio/epinio-server:%s", tag))
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)

		By("Getting the old values ...")
		out, err = proc.RunW("helm", "get", "values", "epinio",
			"-n", "epinio", "-o", "yaml",
		)
		Expect(err).NotTo(HaveOccurred(), out)
		tmpDir, err := os.MkdirTemp("", "helm")
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(path.Join(tmpDir, "values.yaml"), []byte(out), 0644)
		Expect(err).ToNot(HaveOccurred())

		// Upgrade Epinio using the fresh image
		By("Upgrading ...")
		out, err = proc.RunW("helm", "upgrade", "epinio",
			"-n", "epinio",
			"../../helm-charts/chart/epinio",
			"-f", path.Join(tmpDir, "values.yaml"),
			"--set", "image.epinio.registry=ghcr.io/",
			"--set", fmt.Sprintf("image.epinio.tag=%s", tag),
			"--wait",
		)
		Expect(err).NotTo(HaveOccurred(), out)

		// Check that the app is still reachable
		By("Checking reachability ...")
		Eventually(func() int {
			resp, err := env.Curl("GET", route, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			return resp.StatusCode
		}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

		// We can think about adding more checks later like application with
		// environment vars or configurations

	})
})
