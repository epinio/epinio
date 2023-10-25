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
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(FailWithReport)
	RunSpecs(t, "Acceptance Suite")
}

var (
	nodeTmpDir string
	// Lets see if ok with init
	env testenv.EpinioEnv
)

func InstallCertManager() {
	out, err := proc.RunW("helm", "repo", "add", "jetstack", "https://charts.jetstack.io")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "repo", "update")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "upgrade", "--install", "cert-manager", "jetstack/cert-manager",
		"-n", "cert-manager",
		"--create-namespace",
		"--set", "installCRDs=true",
		"--set", "extraArgs[0]=--enable-certificate-owner-ref=true",
		"--wait",
	)
	Expect(err).NotTo(HaveOccurred(), out)
}

func InstallNginx() {
	out, err := proc.RunW("helm", "repo", "add", "ingress-nginx", "https://kubernetes.github.io/ingress-nginx")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "repo", "update")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "upgrade", "--install", "ingress-nginx", "ingress-nginx/ingress-nginx",
		"--namespace", "ingress-nginx",
		"--create-namespace",
		"--set", "controller.ingressClassResource.default=true",
	)
	Expect(err).NotTo(HaveOccurred(), out)
}

func InstallTraefik() {
	out, err := proc.RunW("helm", "repo", "add", "traefik", "https://traefik.github.io/charts", "--force-update")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "repo", "update")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "upgrade", "--install", "traefik", "traefik/traefik",
		"-n", "traefik",
		"--create-namespace",
		"--set", "ports.web.redirectTo.port=websecure",
		"--set", "ingressClass.enabled=true",
		"--set", "ingressClass.isDefaultClass=true",
	)
	Expect(err).NotTo(HaveOccurred(), out)
}

func UpgradeSequence(epinioHelper epinio.Epinio, domain string) {
	By("Upgrading", func() {
		var (
			// after*  - after upgrade
			// before* - before upgrade

			afterApp     string // Application
			afterCatalog string // Service Catalog
			afterConfig  string // Configuration
			afterRoute   string // App route
			afterService string // Service

			beforeApp     string // Same as above, before upgrade
			beforeCatalog string
			beforeConfig  string
			beforeRoute   string
			beforeService string

			namespace string // Namespace (created before upgrade)
			catentry  string // Service catalog entry to use
		)

		By("Setup And Checks Before Upgrade", func() {
			catentry = "mysql-dev"
			namespace = catalog.NewNamespaceName()

			env.SetupAndTargetNamespace(namespace)

			beforeApp = catalog.NewAppName()
			afterApp = catalog.NewAppName()
			afterService = catalog.NewServiceNamePrefixed("after")
			beforeService = catalog.NewServiceNamePrefixed("before")
			afterConfig = catalog.NewConfigurationName()
			beforeConfig = catalog.NewConfigurationName()
			beforeCatalog = catalog.NewCatalogServiceNamePrefixed("before")
			afterCatalog = catalog.NewCatalogServiceNamePrefixed("after")

			// Note current versions of client and server
			By("Versions before upgrade")
			env.Versions()

			// TODO - remove once v1.11.0 is released - temporary workaround for upgrade tests
			By("Write Procfile for golang-sample-app pre-upgrade (needed for v1.10.0)", func() {
				currentDir, err := os.Getwd()
				ExpectWithOffset(1, err).ToNot(HaveOccurred())
				// currentDir is ~/actions-runner/_work/epinio/epinio/acceptance/install
				appDir := currentDir + "/../../assets/golang-sample-app"
				procfile_content := "web: golang-sample-app\n"
				procfile_filePath := appDir + "/Procfile"

				file, err := os.Create(procfile_filePath)
				Expect(err).NotTo(HaveOccurred())
				defer file.Close()

				_, err = file.WriteString(procfile_content)
				Expect(err).NotTo(HaveOccurred())
			})

			// Deploy a simple application before upgrading Epinio, check that it is reachable
			By("Deploy application pre-upgrade")
			env.MakeGolangApp(beforeApp, 1, true)
			beforeRoute = fmt.Sprintf("https://%s.%s", beforeApp, domain)
			By("Route: " + beforeRoute)
			Eventually(func() int {
				resp, err := env.Curl("GET", beforeRoute, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				return resp.StatusCode
			}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

			// Check that we can create a configuration before the upgrade
			By("Create configuration pre-upgrade")
			out, err := env.Epinio("", "configuration", "create", beforeConfig, "fox", "lair")
			Expect(err).ToNot(HaveOccurred(), out)

			// Check that we can create a service before the upgrade
			By("Create service pre-upgrade")
			env.MakeServiceInstance(beforeService, catentry)

			// Check that a custom catalog entry is visible
			By("Create custom catalog entry pre-upgrade")
			catalog.CreateCatalogService(catalog.NginxCatalogService(beforeCatalog))
			out, err = env.Epinio("", "service", "catalog")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring(beforeCatalog))
		})

		By("Upgrading actual", func() {
			// Update CRDs prior the upgrade
			out, err := proc.Kubectl("apply",
				"-f", "https://raw.githubusercontent.com/epinio/helm-charts/main/chart/epinio/crds/app-crd.yaml",
				"-f", "https://raw.githubusercontent.com/epinio/helm-charts/main/chart/epinio/crds/appcharts-crd.yaml",
				"-f", "https://raw.githubusercontent.com/epinio/helm-charts/main/chart/epinio/crds/service-crd.yaml",
			)
			Expect(err).ToNot(HaveOccurred(), out)
			// Upgrade to current as found in checkout
			epinioHelper.Upgrade()
		})

		By("Checks After Upgrade", func() {
			// Note post-upgrade versions of client and server
			By("Versions after upgrade")
			env.Versions()

			// Check that the before app is still reachable
			By("Checking reachability ...")
			Eventually(func() int {
				resp, err := env.Curl("GET", beforeRoute, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				return resp.StatusCode
			}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

			// Check that the before configuration still exists
			By("Checking configuration existence ...")
			env.HaveConfiguration(beforeConfig)

			By("Checking usability of old service", func() {
				// Check that the before service instance still exists
				By("Checking service existence ...")
				env.HaveServiceInstance(beforeService)

				// Alternate check of the same using `service list` instead of `.. show`.
				out, err := env.Epinio("", "service", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(beforeService))

				// We should be able to bind and unbind the old service to/from an application.

				out, err = env.Epinio("", "service", "bind", beforeService, beforeApp)
				Expect(err).ToNot(HaveOccurred(), out)

				out, err = env.Epinio("", "service", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(HaveATable(WithRow(beforeService, WithDate(), catentry, ".*", "deployed", beforeApp)))

				out, err = env.Epinio("", "service", "unbind", beforeService, beforeApp)
				Expect(err).ToNot(HaveOccurred(), out)

				out, err = env.Epinio("", "service", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(HaveATable(WithRow(beforeService, WithDate(), catentry, ".*", "deployed", "")))
			})

			By("Create configuration post-upgrade")
			out, err := env.Epinio("", "configuration", "create", afterConfig, "dog", "house")
			Expect(err).ToNot(HaveOccurred(), out)

			By("Create service post-upgrade")
			env.MakeServiceInstance(afterService, catentry)

			By("Create custom catalog entry post-upgrade")
			catalog.CreateCatalogService(catalog.NginxCatalogService(afterCatalog))
			out, err = env.Epinio("", "service", "catalog")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring(afterCatalog))

			// TODO - remove once v1.11.0 is released - temporary workaround for upgrade tests
			By("Remove Procfile from golang-sample-app post-upgrade (needed for v1.10.0)", func() {
				currentDir, err := os.Getwd()
				ExpectWithOffset(1, err).ToNot(HaveOccurred())
				// currentDir is ~/actions-runner/_work/epinio/epinio/acceptance/install
				appDir := currentDir + "/../../assets/golang-sample-app"
				procfile_filePath := appDir + "/Procfile"

				err = os.Remove(procfile_filePath)
				Expect(err).NotTo(HaveOccurred())
			})

			// Check that we can create an application after the upgrade, incl. reachability
			By("Create application post-upgrade")
			env.MakeGolangApp(afterApp, 1, true)
			afterRoute = fmt.Sprintf("https://%s.%s", afterApp, domain)
			By("Route: " + afterRoute)
			Eventually(func() int {
				resp, err := env.Curl("GET", afterRoute, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				return resp.StatusCode
			}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))
		})

		By("Teardown After Upgrade", func() {
			env.DeleteApp(beforeApp)
			env.DeleteApp(afterApp)
			env.DeleteService(afterService)
			env.DeleteService(beforeService)
			env.DeleteConfigurations(afterConfig)
			env.DeleteConfigurations(beforeConfig)
			catalog.DeleteCatalogService(afterCatalog)
			catalog.DeleteCatalogService(beforeCatalog)
			env.DeleteNamespace(namespace)
		})
	})
}

var _ = SynchronizedBeforeSuite(func() []byte {
	ingressController := os.Getenv("INGRESS_CONTROLLER")

	By("Installing and configuring the prerequisites", func() {
		testenv.SetRoot("../..")
		testenv.SetupEnv()

		env = testenv.New(nodeTmpDir, testenv.Root(), "admin", "password", "", "")
	})

	released := os.Getenv("EPINIO_RELEASED")
	isreleased := released == "true"

	upgraded := os.Getenv("EPINIO_UPGRADED")
	isupgraded := upgraded == "true"

	if isupgraded || isreleased {
		By("Expecting a client binary")
	} else {
		By("Compiling Epinio", func() {
			testenv.BuildEpinio()
		})
	}

	By("Creating registry secret", func() {
		testenv.CreateRegistrySecret()
	})

	By("Installing cert-manager", func() {
		InstallCertManager()
	})

	By("Installing ingress controller", func() {
		if ingressController == "nginx" {
			fmt.Printf("Using nginx\n")
			InstallNginx()
		} else if ingressController == "traefik" {
			fmt.Printf("Using traefik\n")
			InstallTraefik()
		}
	})

	return []byte{}
}, func(_ []byte) {
	testenv.SetRoot("../..")
	testenv.SetupEnv()

	Expect(os.Getenv("KUBECONFIG")).ToNot(BeEmpty(), "KUBECONFIG environment variable should not be empty")
})

var _ = SynchronizedAfterSuite(func() {
}, func() { // Runs only on one node after all are done
	if testenv.SkipCleanup() {
		fmt.Printf("Found '%s', skipping all cleanup", testenv.SkipCleanupPath())
	} else {
		// Delete left-overs no matter what
		defer func() { _, _ = testenv.CleanupTmp() }()
	}
})

var _ = AfterEach(func() {
	testenv.AfterEachSleep()
})

func FailWithReport(message string, callerSkip ...int) {
	// NOTE: Use something like the following if you need to debug failed tests
	// fmt.Println("\nA test failed. You may find the following information useful for debugging:")
	// fmt.Println("The cluster pods: ")
	// out, err := proc.Kubectl("get pods --all-namespaces")
	// if err != nil {
	// 	fmt.Print(err.Error())
	// } else {
	// 	fmt.Print(out)
	// }

	// Ensures the correct line numbers are reported
	Fail(message, callerSkip[0]+1)
}
