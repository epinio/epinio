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

package v1_test

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/testenv"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServicePortForward Endpoint", LService, func() {
	var namespace string

	var catalogService models.CatalogService
	var catalogServiceURL string
	var catalogServiceHostname string

	Context("With ensured namespace", func() {

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			settings, err := env.GetSettingsFrom(testenv.EpinioYAML())
			Expect(err).ToNot(HaveOccurred())

			catalogServiceName := catalog.NewCatalogServiceName()
			catalogServiceHostname = strings.Replace(settings.API, `https://epinio`, catalogServiceName, 1)

			catalogService = catalog.NginxCatalogService(catalogServiceName)
			catalog.CreateCatalogService(catalogService)

			catalogServiceURL = "http://" + catalogServiceHostname

			DeferCleanup(func() {
				catalog.DeleteCatalogService(catalogService.Meta.Name)
				env.DeleteNamespace(namespace)
			})
		})

		Context("With ensured service", func() {
			var serviceName string

			BeforeEach(func() {
				serviceName = catalog.NewServiceName()

				out, err := env.Epinio("", "service", "create",
					catalogService.Meta.Name, serviceName,
					"--chart-value", "ingress.enabled=true",
					"--chart-value", "ingress.hostname="+catalogServiceHostname)
				Expect(err).ToNot(HaveOccurred(), out)

				// wait for the service to be ready
				Eventually(func() int {
					resp, err := http.Get(catalogServiceURL)
					Expect(err).ToNot(HaveOccurred())
					return resp.StatusCode
				}, "1m", "1s").Should(Equal(http.StatusOK))

				DeferCleanup(func() {
					catalog.DeleteService(serviceName, namespace)
				})
			})

			It("tests the port-forward API", func() {
				By("assemble url")
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s/portforward", serverURL, v1.WsRoot, namespace, serviceName)
				portForwardURL, err := url.Parse(endpoint)
				Expect(err).ToNot(HaveOccurred())

				token, err := authToken()
				Expect(err).ToNot(HaveOccurred())

				values := portForwardURL.Query()
				values.Add("authtoken", token)
				portForwardURL.RawQuery = values.Encode()
				portForwardURL.Scheme = "wss"

				c, _, err := websocket.DefaultDialer.Dial(portForwardURL.String(), nil)
				Expect(err).ToNot(HaveOccurred())

				req, _ := http.NewRequest(http.MethodGet, "http://localhost", nil)
				Expect(req.Write(c.UnderlyingConn())).ToNot(HaveOccurred())

				resp, err := http.ReadResponse(bufio.NewReader(c.UnderlyingConn()), req)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				err = c.Close()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
