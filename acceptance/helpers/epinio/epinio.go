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

	tag := os.Getenv("EPINIO_CURRENT_TAG")
	By("Tag: " + tag)

	By("Building server image ...")
	out, err := proc.Run("../..", false, "docker", "build", "-t", "epinio/epinio-server",
		"-f", "images/Dockerfile", ".")
	Expect(err).NotTo(HaveOccurred(), out)
	By(out)

	By("Building unpacker image ...")
	out, err = proc.Run("../..", false, "docker", "build", "-t", "epinio/epinio-unpacker",
		"-f", "images/unpacker-Dockerfile", ".")
	Expect(err).NotTo(HaveOccurred(), out)
	By(out)

	local := "epinio/epinio-server"
	remote := fmt.Sprintf("ghcr.io/%s:%s", local, tag)

	localPacker := "epinio/epinio-unpacker"
	remotePacker := fmt.Sprintf("ghcr.io/%s:%s", localPacker, tag)

	By("Image: " + remote)
	By("Image: " + remotePacker)

	if os.Getenv("PUBLIC_CLOUD") == "" {
		// Local k3ds/k3d-based cluster. Talk directly to it. Import the new images into k3d
		By("Importing server image ...")
		out, err = proc.RunW("k3d", "image", "import", "-c", "epinio-acceptance", remote)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)

		By("Importing unpacker image ...")
		out, err = proc.RunW("k3d", "image", "import", "-c", "epinio-acceptance", remotePacker)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)
	} else {
		By("Pushing server image to GHCR ...")
		// PUBLIC_CLOUD is present
		// Pushing new images into ghcr for the public cluster to pull from
		out, err = proc.RunW("docker", "tag", local+":latest", remote)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)

		out, err = proc.RunW("docker", "push", remote)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)

		out, err = proc.RunW("docker", "tag", localPacker+":latest", remotePacker)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)

		out, err = proc.RunW("docker", "push", remotePacker)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)
	}

	By("Upgrading server side")
	out, err = proc.RunW("helm", "upgrade", "--reuse-values", "epinio",
		"-n", "epinio",
		"../../helm-charts/chart/epinio",
		"--set", "image.epinio.registry=ghcr.io/",
		"--set", fmt.Sprintf("image.epinio.tag=%s", tag),
		"--set", fmt.Sprintf("image.bash.tag=%s", tag),
		"--wait",
	)
	Expect(err).NotTo(HaveOccurred(), out)

	By("... Upgrade complete")
}

func (e *Epinio) Install(args ...string) (string, error) {

	// Default is install from local chart
	chart := "helm-charts/chart/epinio"

	// If requested by the environment, switch to latest release instead, which is older
	released := os.Getenv("EPINIO_RELEASED")
	isreleased := released == "true"
	upgraded := os.Getenv("EPINIO_UPGRADED")
	isupgraded := upgraded == "true"

	if isupgraded || isreleased {
		out, err := proc.RunW("helm", "repo", "add", "epinio", "https://epinio.github.io/helm-charts")
		if err != nil {
			return out, err
		}
		chart = "epinio/epinio"
	}

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
		chart,
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
