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

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/testenv"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServicePortForward Endpoint", Label("ServicePortForward"), func() {
	var epinioHelper epinio.Epinio
	var namespace string
	var catalogService models.CatalogService

	Context("With ensured namespace", func() {
		BeforeEach(func() {
			epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)
			catalogService = catalog.CreateCatalogServiceApache()
		})

		AfterEach(func() {
			catalog.DeleteCatalogService(catalogService.Meta.Name)
			env.DeleteNamespace(namespace)
		})

		Context("With ensured service", func() {
			var serviceName string

			BeforeEach(func() {
				serviceName = catalog.NewServiceName()
				By("Deploying Apache with service", func() {
					out, err := epinioHelper.Run("service", "create", "apache-test", serviceName, "--wait")
					Expect(err).ToNot(HaveOccurred(), out)
				})
			})

			AfterEach(func() {
				catalog.DeleteService(serviceName, namespace)
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

				defer c.Close()
			})
		})
	})
})
