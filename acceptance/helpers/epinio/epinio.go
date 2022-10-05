package epinio

import (
	"fmt"
	"os"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type Epinio struct {
	EpinioBinaryPath string
}

func NewEpinioHelper(epinioBinaryPath string) Epinio {
	return Epinio{
		EpinioBinaryPath: epinioBinaryPath,
	}
}

func (e *Epinio) Run(cmd string, args ...string) (string, error) {
	out, err := proc.RunW(e.EpinioBinaryPath, append([]string{cmd}, args...)...)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (e *Epinio) Upgrade() {
	// Redundant, done by prepare_environment_k3d in flow setup (upgrade.yml)
	// Build image with latest epinio-server binary
	By("Upgrading ...")

	By("Rebuilding client side")
	testenv.BuildEpinio()

	// NOTE: Client has to be rebuild first. Because its result is also what goes into the
	// server image to be assembled in the coming step below. Not doing so causes the new server
	// image to contain the old binary.

	By("Building server image ...")
	out, err := proc.Run("../..", false, "docker", "build", "-t", "epinio/epinio-server",
		"-f", "images/Dockerfile", ".")
	Expect(err).NotTo(HaveOccurred(), out)
	By(out)

	tag := os.Getenv("EPINIO_CURRENT_TAG")
	By("Tag: " + tag)

	local := "epinio/epinio-server"
	remote := fmt.Sprintf("ghcr.io/%s:%s", local, tag)
	By("Image: " + remote)

	if os.Getenv("PUBLIC_CLOUD") == "" {
		// Local k3ds/k3d-based cluster. Talk directly to it. Import the new image into k3d
		By("Importing server image ...")
		out, err = proc.RunW("k3d", "image", "import", "-c", "epinio-acceptance", remote)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)
	} else {
		By("Pushing server image to GHCR ...")
		// PUBLIC_CLOUD is present
		// Pushing new image into ghcr for the public cluster to pull from
		out, err = proc.RunW("docker", "tag", local+":latest", remote)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)

		out, err = proc.RunW("docker", "push", remote)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)
	}

	By("Upgrading server side")
	out, err = proc.RunW("helm", "upgrade", "--reuse-values", "epinio",
		"-n", "epinio",
		"../../helm-charts/chart/epinio",
		"--set", "image.epinio.registry=ghcr.io/",
		"--set", fmt.Sprintf("image.epinio.tag=%s", tag),
		"--wait",
	)
	Expect(err).NotTo(HaveOccurred(), out)

	By("... Upgrade complete")
}

func (e *Epinio) Install(args ...string) (string, error) {
	// Update helm repos -- Assumes existence of helm repository providing the helm charts
	out, err := proc.RunW("helm", "repo", "update")
	if err != nil {
		return out, err
	}

	opts := []string{
		"upgrade",
		"--install",
		"-n",
		"epinio",
		"--create-namespace",
		"epinio",
		"helm-charts/chart/epinio",
		"--wait",
	}

	out, err = proc.Run(testenv.Root(), false, "helm", append(opts, args...)...)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (e *Epinio) Uninstall() (string, error) {
	out, err := proc.RunW("helm", "uninstall", "-n", "epinio", "epinio")
	if err != nil {
		return out, err
	}
	return out, nil
}
