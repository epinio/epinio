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
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/helpers/route53"
	"github.com/epinio/epinio/acceptance/testenv"
)

var _ = Describe("<Scenario8> RKE, Private CA, Configuration, on External Registry, S3 storage", func() {
	var (
		flags             []string
		epinioHelper      epinio.Epinio
		configurationName = catalog.NewConfigurationName()
		appName           string
		loadbalancer      string
		registryUsername  string
		registryPassword  string
		domain            string
		zoneID            string
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
		// domainIP = strings.TrimSuffix(domain, ".omg.howdoi.website")

		appName = "external-reg-test-rke"

		registryUsername = os.Getenv("REGISTRY_USERNAME")
		Expect(registryUsername).ToNot(BeEmpty())

		registryPassword = os.Getenv("REGISTRY_PASSWORD")
		Expect(registryPassword).ToNot(BeEmpty())

		accessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
		Expect(accessKeyID).ToNot(BeEmpty())

		secretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
		Expect(secretAccessKey).ToNot(BeEmpty())

		flags = []string{
			"--set", "server.disableTracking=true", // disable tracking during tests
			"--set", "global.domain=" + domain,
			"--set", "global.tlsIssuer=private-ca",
			"--set", "containerregistry.enabled=false",
			"--set", "global.registryURL=registry.hub.docker.com",
			"--set", "global.registryUsername=" + registryUsername,
			"--set", "global.registryPassword=" + registryPassword,
			"--set", "global.registryNamespace=splatform",
			"--set", "minio.enabled=false",
			"--set", "s3gw.enabled=false",
			"--set", "s3.useSSL=true",
			"--set", "s3.bucket=epinio-ci",
			"--set", "s3.endpoint=s3.eu-central-1.amazonaws.com",
			"--set", "s3.region=eu-central-1",
			"--set", "s3.accessKeyID=" + accessKeyID,
			"--set", "s3.secretAccessKey=" + secretAccessKey,
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
		By("Checking LoadBalancer IP", func() {
			// Ensure that Nginx LB is not in Pending state anymore, could take time
			Eventually(func() string {
				out, err := proc.RunW("kubectl", "get", "svc", "-n", "ingress-nginx", "ingress-nginx-controller", "--no-headers")
				Expect(err).NotTo(HaveOccurred(), out)
				return out
			}, "4m", "2s").ShouldNot(ContainSubstring("<pending>"))

			out, err := proc.RunW("kubectl", "get", "service", "-n", "ingress-nginx", "ingress-nginx-controller", "-o", "json")
			Expect(err).NotTo(HaveOccurred(), out)

			// Check that an IP address for LB is configured
			status := &testenv.LoadBalancerHostname{}
			err = json.Unmarshal([]byte(out), status)
			Expect(err).NotTo(HaveOccurred())
			Expect(status.Status.LoadBalancer.Ingress).To(HaveLen(1))
			loadbalancer = status.Status.LoadBalancer.Ingress[0].Hostname
			Expect(loadbalancer).ToNot(BeEmpty())
		})

		By("Updating DNS Entries", func() {
			change := route53.CNAME(domain, loadbalancer, "UPSERT")
			out, err := route53.Update(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)

			change = route53.CNAME("*."+domain, loadbalancer, "UPSERT")
			out, err = route53.Update(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)
		})

		// Check that DNS entry is correctly propagated
		By("Checking that DNS entry is correctly propagated", func() {
			Eventually(func() string {
				out, err := route53.TestDnsAnswer(zoneID, domain, "CNAME")
				Expect(err).NotTo(HaveOccurred(), out)

				answer := &route53.DNSAnswer{}
				err = json.Unmarshal([]byte(out), answer)
				Expect(err).NotTo(HaveOccurred())
				if len(answer.RecordData) == 0 {
					return ""
				}
				return answer.RecordData[0]
			}, "5m", "2s").Should(Equal(loadbalancer + ".")) // CNAME ends with a '.'
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

		// Workaround to (try to!) ensure that the DNS is really propagated!
		time.Sleep(3 * time.Minute)

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
