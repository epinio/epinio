// Copyright © 2021 - 2026 SUSE LLC
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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/helpers/harbor"
	"github.com/epinio/epinio/acceptance/testenv"
)

// EPINIO-823: Harbor-only acceptance for application charts (not Docker Hub).
// Automates the Harbor chart push / use / delete path that was verified
// manually around the 1.14.3 external-registry work.
var _ = Describe("<ScenarioHarbor> Harbor as external registry for app charts", Serial, func() {
	const chartArchiveURL = "https://github.com/epinio/helm-charts/releases/download/" +
		"epinio-application-0.1.26/epinio-application-0.1.26.tgz"

	var (
		epinioHelper epinio.Epinio
		domain       string
		chartName    string
		chartArchive string
		namespace    string
		registryHost string
		appName      string
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		domain = os.Getenv("EPINIO_SYSTEM_DOMAIN")
		Expect(domain).ToNot(BeEmpty(), "EPINIO_SYSTEM_DOMAIN must be set")

		chartName = catalog.NewTmpName("harbor-chart-")
		namespace = catalog.NewNamespaceName()
		appName = catalog.NewAppName()

		By("downloading a packaged application chart")
		chartArchive = catalog.DownloadChartArchive(chartArchiveURL)
	})

	AfterEach(func() {
		_, _ = epinioHelper.Run("apps", "delete", appName)
		_, _ = epinioHelper.Run("app", "chart", "delete", chartName)
		_, _ = epinioHelper.Run("namespace", "delete", namespace)
		_, _ = epinioHelper.Uninstall()
		harbor.Uninstall()
		harbor.DeleteExternalRegistryTLS()
	})

	It("pushes, shows, uses, and deletes an app chart stored in Harbor", func() {
		By("Installing Harbor on ingress harbor." + domain)
		registryHost = harbor.Install(domain)

		By("Publishing Harbor's TLS cert for Epinio registry verification")
		harbor.CopyCAToEpinio()

		By("Installing Epinio with Harbor as the external registry")
		// Server-side helm deploy uses server.timeoutMultiplier (default 1 → 3m).
		// CLI EPINIO_TIMEOUT_MULTIPLIER alone does not cover that; Harbor image
		// pull after staging often needs longer on k3d / cold CI runners.
		timeoutMultiplier := os.Getenv("EPINIO_TIMEOUT_MULTIPLIER")
		if timeoutMultiplier == "" {
			timeoutMultiplier = "5"
		}
		flags := []string{
			"--set", "global.domain=" + domain,
			"--set", "ingress.tls.enabled=true",
			"--set", "containerregistry.enabled=false",
			"--set", "global.registryURL=" + registryHost,
			"--set", "global.registryNamespace=" + harbor.ProjectName,
			"--set", "global.registryUsername=" + harbor.AdminUser,
			"--set", "global.registryPassword=" + harbor.AdminPassword(),
			"--set", "containerregistry.certificateSecret=" + harbor.ExternalRegistryTLSSecret,
			"--set", "server.timeoutMultiplier=" + timeoutMultiplier,
		}
		if extraEnvName, nameOK := os.LookupEnv("EXTRAENV_NAME"); nameOK {
			if extraEnvValue, valueOK := os.LookupEnv("EXTRAENV_VALUE"); valueOK {
				flags = append(flags,
					"--set", "extraEnv[0].name="+extraEnvName,
					"--set-string", "extraEnv[0].value="+extraEnvValue,
				)
			}
		}

		out, err := epinioHelper.Install(flags...)
		Expect(err).NotTo(HaveOccurred(), out)
		Expect(out).To(ContainSubstring("STATUS: deployed"))

		out, err = testenv.PatchEpinio()
		Expect(err).ToNot(HaveOccurred(), out)

		By("Connecting to Epinio")
		loginURL := "https://epinio." + domain
		// k3d publishes HTTPS on EPINIO_PORT (usually 8443), not 443.
		if p := os.Getenv("EPINIO_PORT"); p != "" {
			loginURL = loginURL + ":" + p
		}
		Eventually(func() string {
			out, _ := epinioHelper.Run("login", "-u", "admin", "-p", "password", "--trust-ca",
				loginURL)
			return out
		}, "2m", "5s").Should(ContainSubstring("Login successful"))

		By("Pushing an application chart into Harbor via Epinio")
		out, err = epinioHelper.Run("app", "chart", "push", chartName, chartArchive,
			"--short-description", "harbor acceptance chart",
			"--description", "EPINIO-823 Harbor app chart acceptance")
		Expect(err).NotTo(HaveOccurred(), out)
		Expect(out).To(ContainSubstring("Application Chart Pushed"))
		Expect(out).To(ContainSubstring(chartName))
		Expect(out).To(ContainSubstring("oci://" + registryHost + "/" + harbor.ProjectName))
		Expect(out).NotTo(ContainSubstring(".svc.cluster.local"))

		By("Confirming the chart repository exists in Harbor's API")
		harbor.ExpectAppChartRepo(registryHost, chartName)

		By("Showing the chart references Harbor's OCI repository")
		out, err = epinioHelper.Run("app", "chart", "show", chartName)
		Expect(err).NotTo(HaveOccurred(), out)
		Expect(out).To(ContainSubstring(chartName))
		Expect(out).To(ContainSubstring("oci://" + registryHost + "/" + harbor.ProjectName))
		Expect(out).To(ContainSubstring("epinio_appchart_"))

		By("Creating a namespace and deploying an app with the Harbor-backed chart")
		out, err = epinioHelper.Run("namespace", "create", namespace)
		Expect(err).NotTo(HaveOccurred(), out)
		out, err = epinioHelper.Run("target", namespace)
		Expect(err).NotTo(HaveOccurred(), out)

		out, err = epinioHelper.Run("push",
			"--name", appName,
			"--path", testenv.AssetPath("sample-app"),
			"--app-chart", chartName,
		)
		Expect(err).NotTo(HaveOccurred(), out)

		By("Confirming the deployed app references the Harbor-backed chart")
		out, err = epinioHelper.Run("app", "show", appName)
		Expect(err).NotTo(HaveOccurred(), out)
		Expect(out).To(ContainSubstring(chartName))

		By("Deleting the application and its Harbor-backed chart")
		out, err = epinioHelper.Run("apps", "delete", appName)
		Expect(err).NotTo(HaveOccurred(), out)

		out, err = epinioHelper.Run("app", "chart", "delete", chartName)
		Expect(err).NotTo(HaveOccurred(), out)

		out, err = epinioHelper.Run("app", "chart", "show", chartName)
		Expect(err).To(HaveOccurred(), out)
		Expect(out).To(ContainSubstring("does not exist"))
	})
})
