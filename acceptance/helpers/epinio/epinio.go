// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	serverTag := fmt.Sprintf("ghcr.io/epinio/epinio-server:%s", tag)
	out, err := proc.Run("../..", false, "docker", "build", "-t", serverTag, "-f", "images/Dockerfile", ".")
	Expect(err).NotTo(HaveOccurred(), out)
	By(out)

	By("Building unpacker image ...")
	unpackerTag := fmt.Sprintf("ghcr.io/epinio/epinio-unpacker:%s", tag)
	out, err = proc.Run("../..", false, "docker", "build", "-t", unpackerTag, "-f", "images/unpacker-Dockerfile", ".")
	Expect(err).NotTo(HaveOccurred(), out)
	By(out)

	By("Image: " + serverTag)
	By("Image: " + unpackerTag)

	if os.Getenv("PUBLIC_CLOUD") == "" {
		// Local k3ds/k3d-based cluster. Talk directly to it. Import the new images into k3d
		By("Importing server image ...")
		out, err = proc.RunW("k3d", "image", "import", "-c", "epinio-acceptance", serverTag)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)

		By("Importing unpacker image ...")
		out, err = proc.RunW("k3d", "image", "import", "-c", "epinio-acceptance", unpackerTag)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)
	} else {
		By("Pushing server image to GHCR ...")
		// PUBLIC_CLOUD is present
		// Pushing new images into ghcr for the public cluster to pull from
		out, err = proc.RunW("docker", "push", serverTag)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)

		out, err = proc.RunW("docker", "push", unpackerTag)
		Expect(err).NotTo(HaveOccurred(), out)
		By(out)
	}

	// If the chart has new defaults Helm will not get these values when using --set and --reuse-values
	// To get the new defaults, the previous values and the --set we need to fetch the values and pass them.
	// See: https://shipmight.com/blog/understanding-helm-upgrade-reset-reuse-values
	// also: https://github.com/helm/helm/issues/8085

	By("Get old Helm values")
	prevValuesFile := "prev-values.yaml"

	out, err = proc.RunW("helm", "get", "values", "epinio", "-n", "epinio", "-o", "yaml")
	Expect(err).NotTo(HaveOccurred(), out)
	By("Old values = ((" + out + "))")
	err = os.WriteFile(prevValuesFile, []byte(out), 0600)
	Expect(err).NotTo(HaveOccurred(), out)

	By("Upgrading server side")
	out, err = proc.RunW("helm", "upgrade", "epinio",
		"-n", "epinio",
		"../../helm-charts/chart/epinio",
		"--values", prevValuesFile,
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
		"--set", "server.disableTracking=true", // disable tracking during tests
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
