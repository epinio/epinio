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

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/testenv"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("<Upgrade2> Epinio upgrade with bound app and services", func() {
	var (
		namespace   string // Namespace created before upgrade
		appName     string // Application created before upgrade
		serviceName string // Service created after upgrade

		epinioHelper epinio.Epinio
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		// Before upgrade ...
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()
		serviceName = catalog.NewServiceName()

		DeferCleanup(func() {
			env.DeleteNamespace(namespace)
		})
	})

	It("Can upgrade epinio bound to a custom service", func() {

		// Note current versions of client and server
		By("Versions before upgrade")
		env.Versions()

		// Create a service
		By("Creating a service")
		out, err := env.Epinio("", "service", "create", "mysql-dev", serviceName)
		Expect(err).ToNot(HaveOccurred(), out)

		By("Wait for deployment")
		// Make outValue global variable as we'll need it later
		var outValue string
		Eventually(func() string {
			outValue, _ = env.Epinio("", "service", "show", serviceName)
			return outValue
		}, "2m", "5s").Should(
			HaveATable(
				WithHeaders("KEY", "VALUE"),
				WithRow("Status", "deployed"),
			),
		)
		// Store the service route
		svcRouteRegexp := regexp.MustCompile(`\b(\w{29}-mysql)`)
		svcRoute := string(svcRouteRegexp.Find([]byte(outValue)))

		// Deploy Wordpress application
		By("Pushing Wordpress App")
		wordpress := "https://github.com/epinio/example-wordpress,main"

		pushLog, err := env.EpinioPush("",
			appName,
			"--name", appName,
			"--builder-image", "paketobuildpacks/builder:0.2.443-full",
			"--git", wordpress,
			"-e", "BP_PHP_WEB_DIR=wordpress",
			"-e", "BP_PHP_VERSION=8.0.x",
			"-e", "BP_PHP_SERVER=nginx",
			"-e", "DB_HOST="+svcRoute,
			"-e", "SERVICE_NAME="+serviceName)
		Expect(err).ToNot(HaveOccurred(), pushLog)

		routeRegexp := regexp.MustCompile(`https:\/\/.*sslip.io`)
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
		out, err = env.Epinio("", "service", "bind", serviceName, appName)
		Expect(err).ToNot(HaveOccurred(), out)

		By("Verify binding")
		appShowOut, err := env.Epinio("", "app", "show", appName)
		Expect(err).ToNot(HaveOccurred())

		Expect(appShowOut).To(
			HaveATable(
				WithHeaders("KEY", "VALUE"),
				WithRow("Bound Configurations", svcRoute),
			),
		)

		// Check that the app is reachable
		Eventually(func() int {
			resp, err := env.Curl("GET", route, nil)
			Expect(err).ToNot(HaveOccurred())
			return resp.StatusCode
		}, "1m", "5s").Should(Equal(http.StatusOK))

		// app available, check that the body contains "WordPress"
		resp, err := env.Curl("GET", route, nil)
		Expect(err).ToNot(HaveOccurred())
		bodyBytes, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(bodyBytes)).To(ContainSubstring("WordPress"))

		// Upgrade to current as found in checkout
		epinioHelper.Upgrade()

		// Note post-upgrade versions of client and server
		By("Versions after upgrade")
		env.Versions()

		// Restarting app
		By("Restarting app")
		out, err = env.Epinio("", "app", "restart", appName)
		Expect(err).ToNot(HaveOccurred(), out)

		// Check that the app is still reachable and expected page tile is reached
		Eventually(func() int {
			resp, err := env.Curl("GET", route, nil)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			if resp.StatusCode == http.StatusOK && strings.Contains(string(bodyBytes), "WordPress") {
				return resp.StatusCode
			}

			return 0
		}, "1m", "5s").Should(Equal(http.StatusOK))
	})
})
