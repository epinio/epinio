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

package install_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
)

var _ = Describe("<Scenario3> RKE, Private CA, Configuration, on External Registry", func() {
	var (
		flags             []string
		epinioHelper      epinio.Epinio
		configurationName = catalog.NewConfigurationName()
		appName           string
		loadbalancer      string
		registryUsername  string
		registryPassword  string
		rangeIP           string
		domain            string
		domainIP          string
		extraEnvName      string
		extraEnvValue     string
		name_exists       bool
		value_exists      bool
		localpathURL      = "https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.20/deploy/local-path-storage.yaml"
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		// Clean previous installed helm repos
		// Done at the beginning because we don't know the runner's state
		out, err := proc.Run(testenv.Root(), false, "bash", "./scripts/remove-helm-repos.sh")
		Expect(err).NotTo(HaveOccurred(), out)

		domain = os.Getenv("EPINIO_SYSTEM_DOMAIN")
		Expect(domain).ToNot(BeEmpty())
		domainIP = strings.TrimSuffix(domain, ".sslip.io")

		appName = "external-reg-test-rke"

		registryUsername = os.Getenv("REGISTRY_USERNAME")
		Expect(registryUsername).ToNot(BeEmpty())

		registryPassword = os.Getenv("REGISTRY_PASSWORD")
		Expect(registryPassword).ToNot(BeEmpty())
		flags = []string{
			"--set", "server.disableTracking=true", // disable tracking during tests
			"--set", "global.domain=" + domain,
			"--set", "global.tlsIssuer=private-ca",
			"--set", "containerregistry.enabled=false",
			"--set", "global.registryURL=registry.hub.docker.com",
			"--set", "global.registryUsername=" + registryUsername,
			"--set", "global.registryPassword=" + registryPassword,
			"--set", "global.registryNamespace=epinio",
		}

		extraEnvName, name_exists = os.LookupEnv("EXTRAENV_NAME")
		extraEnvValue, value_exists = os.LookupEnv("EXTRAENV_VALUE")
		if name_exists && value_exists {
			flags = append(flags, "--set", "extraEnv[0].name="+extraEnvName, "--set-string", "extraEnv[0].value="+extraEnvValue)
		}
	})

	AfterEach(func() {
		out, err := epinioHelper.Uninstall()
		Expect(err).NotTo(HaveOccurred(), out)
	})

	It("Installs with private CA and pushes an app with configuration", func() {
		By("Installing MetalLB", func() {
			rangeIP = os.Getenv("RANGE_IP")
			out, err := proc.RunW("sed", "-i", fmt.Sprintf("s/@IP_RANGE@/%s/", rangeIP),
				testenv.TestAssetPath("resources.yaml"))
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = proc.RunW("helm", "repo", "add", "metallb", "https://metallb.github.io/metallb")
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = proc.RunW("helm", "upgrade", "--install", "--wait", "-n", "metallb",
				"--create-namespace", "metallb", "metallb/metallb")
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = proc.RunW("kubectl", "apply", "-f", testenv.TestAssetPath("resources.yaml"))
			Expect(err).NotTo(HaveOccurred(), out)
		})

		By("Checking LoadBalancer IP", func() {
			// Ensure that Traefik LB is not in Pending state anymore, could take time
			Eventually(func() string {
				out, err := proc.RunW("kubectl", "get", "svc", "-n", "traefik", "traefik", "--no-headers")
				Expect(err).NotTo(HaveOccurred(), out)
				return out
			}, "4m", "2s").ShouldNot(ContainSubstring("<pending>"))

			out, err := proc.RunW("kubectl", "get", "service", "-n", "traefik", "traefik", "-o", "json")
			Expect(err).NotTo(HaveOccurred(), out)

			// Check that an IP address for LB is configured
			status := &testenv.LoadBalancerHostname{}
			err = json.Unmarshal([]byte(out), status)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(status.Status.LoadBalancer.Ingress).To(HaveLen(1))
			loadbalancer = status.Status.LoadBalancer.Ingress[0].IP
			Expect(loadbalancer).ToNot(BeEmpty())

			// We need to be sure that the specified IP is really used
			Expect(loadbalancer).To(Equal(domainIP))
		})

		By("Configuring local-path storage", func() {
			out, err := proc.RunW("kubectl", "apply", "-f", localpathURL)
			Expect(err).NotTo(HaveOccurred(), out)

			value := `{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}`
			out, err = proc.RunW("kubectl", "patch", "storageclass", "local-path", "-p", value)
			Expect(err).NotTo(HaveOccurred(), out)
		})

		By("Creating private CA issuer", func() {
			// Create certificate secret and cluster_issuer
			out, err := proc.RunW("kubectl", "apply", "-f", testenv.TestAssetPath("cluster-issuer-private-ca.yml"))
			Expect(err).NotTo(HaveOccurred(), out)
		})

		By("Installing Epinio", func() {
			out, err := epinioHelper.Install(flags...)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("STATUS: deployed"))

			out, err = testenv.PatchEpinio()
			Expect(err).ToNot(HaveOccurred(), out)
		})

		By("Connecting to Epinio", func() {
			Eventually(func() string {
				out, _ := epinioHelper.Run("login", "-u", "admin", "-p", "password", "--trust-ca", "https://epinio."+domain)
				return out
			}, "2m", "5s").Should(ContainSubstring("Login successful"))
		})

		By("Checking Epinio info command", func() {
			Eventually(func() string {
				out, _ := epinioHelper.Run("info")
				return out
			}, "2m", "2s").Should(ContainSubstring("Epinio Server Version:"))

			env.Versions()
		})

		By("Creating a configuration and pushing an app", func() {
			out, err := epinioHelper.Run("configuration", "create", configurationName, "mariadb", "10-3-22")
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = epinioHelper.Run("push",
				"--name", appName,
				"--path", testenv.AssetPath("sample-app"),
				"--bind", configurationName)
			Expect(err).NotTo(HaveOccurred(), out)

			env.VerifyAppConfigurationBound(appName, configurationName, testenv.DefaultWorkspace, 1)

			// Verify cluster_issuer is used
			out, err = proc.RunW("kubectl", "get", "certificate",
				"-n", testenv.DefaultWorkspace,
				"--selector", "app.kubernetes.io/name="+appName,
				"-o", "jsonpath='{.items[*].spec.issuerRef.name}'")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal("'private-ca'"))
		})

		By("Delete an app", func() {
			out, err := epinioHelper.Run("apps", "delete", appName)
			Expect(err).NotTo(HaveOccurred(), out)

			// We check for both bulk deletion response and old response. Because with
			// upgrade testing the pre-upgrade binary may be without bulk deletion
			// support.
			Expect(out).To(Or(
				ContainSubstring("Applications Removed"),
				ContainSubstring("Application deleted")))
		})

		if os.Getenv("EPINIO_UPGRADED") == "true" {
			UpgradeSequence(epinioHelper, domain)
		}
	})
})
