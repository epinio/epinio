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

var _ = Describe("<Upgrade1> Epinio upgrade with running app", func() {
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

		DeferCleanup(func() {
			env.DeleteNamespace(namespace)
		})
	})

	It("can upgrade epinio", func() {
		// Note current versions of client and server
		By("Versions before upgrade")
		env.Versions()

		// Deploy a simple application before upgrading Epinio
		out := env.MakeGolangApp(appName, 1, true)
		routeRegexp := regexp.MustCompile(`https:\/\/.*sslip.io`)
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
})
