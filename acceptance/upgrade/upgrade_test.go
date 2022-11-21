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

package upgrade_test

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/testenv"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Epinio upgrade with running app", func() {
	var (
		namespace string // Namespace created before upgrade
		appName   string // Application created before upgrade
		service   string // Service created after upgrade
		appAfter  string // application created after upgrade

		epinioHelper epinio.Epinio
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		// Before upgrade ...
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()
		appAfter = catalog.NewAppName()
		service = catalog.NewServiceName()
	})

	AfterEach(func() {
		// After upgrade ...
		env.DeleteApp(appName)
		env.DeleteApp(appAfter)
		env.DeleteService(service)
		env.DeleteNamespace(namespace)
	})

	It("can upgrade epinio", func() {
		// Note current versions of client and server
		By("Versions before upgrade")
		env.Versions()

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

		// Upgrade to current as found in checkout
		epinioHelper.Upgrade()

		// Note post-upgrade versions of client and server
		By("Versions after upgrade")
		env.Versions()

		// Check that the app is still reachable
		By("Checking reachability ...")
		Eventually(func() int {
			resp, err := env.Curl("GET", route, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			return resp.StatusCode
		}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

		// Check that we can create a service after the upgrade
		By("Creating a service post-upgrade")

		out, err := env.Epinio("", "service", "create", "mysql-dev", service)
		Expect(err).ToNot(HaveOccurred(), out)

		By("wait for deployment")
		Eventually(func() string {
			out, _ := env.Epinio("", "service", "show", service)
			return out
		}, "2m", "5s").Should(
			HaveATable(
				WithHeaders("KEY", "VALUE"),
				WithRow("Status", "deployed"),
			),
		)

		// Check that we can create an application after the upgrade
		By("Creating an application post-upgrade")

		out = env.MakeGolangApp(appAfter, 1, true)
		route = string(routeRegexp.Find([]byte(out)))

		// Check that the app is reachable
		Eventually(func() int {
			resp, err := env.Curl("GET", route, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			return resp.StatusCode
		}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

		// We can think about adding more checks later like application with
		// environment vars or configurations
	})

	FIt("Can upgrade epinio binded to a custom service", func() {
		// Test variables
		var myService = "mycustom-service"

		// Note current versions of client and server
		By("Versions before upgrade")
		env.Versions()

		// Create a service
		By("Creating a service")
		out, err := env.Epinio("", "service", "create", "mysql-dev", myService)
		Expect(err).ToNot(HaveOccurred(), out)

		By("Wait for deployment")
		Eventually(func() string {
			out, _ := env.Epinio("", "service", "show", myService)
			return out
		}, "2m", "5s").Should(
			HaveATable(
				WithHeaders("KEY", "VALUE"),
				WithRow("Status", "deployed"),
			),
		)
		time.Sleep(25*time.Second)

		// Deploy Wordpress application
		By("Pushing Wordpress App") 
		wordpress := "https://github.com/epinio/example-wordpress,main"
		pushLog, err := env.EpinioPush("",
			appName,
			"--name", appName,
			"--git", wordpress,
			"-e", "BP_PHP_WEB_DIR=wordpress",
			"-e", "BP_PHP_VERSION=8.0.x",
			"-e", "BP_PHP_SERVER=nginx",
			"-e", "DB_HOST=x8e5ee833a0f2faebaf5c4171baca-mysql",
			"-e", "SERVICE_NAME=mycustom-service")
		time.Sleep(25*time.Second)
		Expect(err).ToNot(HaveOccurred(), pushLog)

		routeRegexp := regexp.MustCompile(`https:\/\/.*omg.howdoi.website`)
		route := string(routeRegexp.Find([]byte(pushLog)))

		Eventually(func() string {
			out, err := env.Epinio("", "app", "list")
			Expect(err).ToNot(HaveOccurred(), out)
			return out
		}, "5m").Should(
			HaveATable(
				WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
				WithRow(appName, WithDate(), "1/1", appName+".*", "", ""),
			),
		)
		
		By("Bind it")
		out, err = env.Epinio("", "service", "bind", myService, appName)
		Expect(err).ToNot(HaveOccurred(), out)
		time.Sleep(10*time.Second)

		By("Verify binding")
				appShowOut, err := env.Epinio("", "app", "show", appName)
				Expect(err).ToNot(HaveOccurred())

				Expect(appShowOut).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Bound Configurations", "x8e5ee833a0f2faebaf5c4171baca-mysql"),
					),
				)		
		time.Sleep(15*time.Second)
		
		// Check that the app is reachable and expected page tile is reached
		Eventually(func() int {
			resp, err := env.Curl("GET", route, nil)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			time.Sleep(15*time.Second)
			
			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
			time.Sleep(10*time.Second)		
			Expect(string(bodyBytes)).To(ContainSubstring("WordPress"))
			
			return resp.StatusCode
		}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

		// Upgrade to current as found in checkout
		epinioHelper.Upgrade()
		
		// Note post-upgrade versions of client and server
		By("Versions after upgrade")
		env.Versions()

		// Restarting app
		By("Restarting app")
		out, err = env.Epinio("", "app", "restart", appName)
		time.Sleep(15*time.Second)
		Expect(err).ToNot(HaveOccurred(), out)

		// Check that the app is still reachable and expected page tile is reached
		Eventually(func() int {
			resp, err := env.Curl("GET", route, nil)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			time.Sleep(15*time.Second)
			
			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
			time.Sleep(10*time.Second)		
			Expect(string(bodyBytes)).To(ContainSubstring("WordPress"))
			
			return resp.StatusCode
		}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))
	})


})
