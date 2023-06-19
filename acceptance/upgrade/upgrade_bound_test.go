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
	"fmt"
	"io"
	"net/http"
	"regexp"

	//"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/testenv"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("<Upgrade2> Epinio upgrade with bound app and services", func() {
	var (
		namespace string // Namespace created before upgrade
		appName   string // Application created before upgrade
		service   string // Service created after upgrade

		epinioHelper epinio.Epinio
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		// Before upgrade ...
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()
		service = catalog.NewServiceName()
	})

	AfterEach(func() {
		if !CurrentSpecReport().Failed() {
			// After upgrade ...
			env.DeleteApp(appName)
			env.DeleteService(service)
			env.DeleteNamespace(namespace)
		}
	})

	It("Can upgrade epinio bound to a custom service", func() {

		// Note current versions of client and server
		By("Versions before upgrade")
		env.Versions()

		// Create a service
		By("Creating a service")
		out, err := env.Epinio("", "service", "create", "mysql-dev", service)
		Expect(err).ToNot(HaveOccurred(), out)

		// Debug only
		fmt.Fprintf(GinkgoWriter, "Service create log: %v\n", out)

		By("Wait for deployment")
		// Make outValue global variable as we'll need it later
		var outValue string
		Eventually(func() string {
			outValue, _ = env.Epinio("", "service", "show", service)
			return outValue
		}, "2m", "5s").Should(
			HaveATable(
				WithHeaders("KEY", "VALUE"),
				WithRow("Status", "deployed"),
			),
		)
		// store the service route
		svcRouteRegexp := regexp.MustCompile(`\b(\w{29}-mysql)`)
		svcRoute := string(svcRouteRegexp.Find([]byte(outValue)))
		fmt.Fprintf(GinkgoWriter, "The service table content: %v\n", outValue)
		fmt.Fprintf(GinkgoWriter, "The service route is: %v\n", svcRoute)

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
			"-e", "DB_HOST="+svcRoute,
			"-e", "SERVICE_NAME="+service)
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
		out, err = env.Epinio("", "service", "bind", service, appName)
		Expect(err).ToNot(HaveOccurred(), out)
		// This should be done as Eventually block
		time.Sleep(10 * time.Second)

		By("Verify binding")
		appShowOut, err := env.Epinio("", "app", "show", appName)
		Expect(err).ToNot(HaveOccurred())

		Expect(appShowOut).To(
			HaveATable(
				WithHeaders("KEY", "VALUE"),
				WithRow("Bound Configurations", svcRoute),
			),
		)

		// Check that the app is reachable and expected page tile is reached
		Eventually(func() int {
			resp, err := env.Curl("GET", route, nil)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
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
		time.Sleep(15 * time.Second)
		Expect(err).ToNot(HaveOccurred(), out)

		// Check that the app is still reachable and expected page tile is reached
		Eventually(func() int {
			resp, err := env.Curl("GET", route, nil)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
			Expect(string(bodyBytes)).To(ContainSubstring("WordPress"))

			return resp.StatusCode
		}, "2m", "5s").Should(Equal(http.StatusOK))
	})
})
