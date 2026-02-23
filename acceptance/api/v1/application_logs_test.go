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
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/testenv"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppLogs Endpoint", LApplication, func() {
	var (
		namespace string
		logLength int
		route     string
		app       string
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		logLength = 0

		app = catalog.NewAppName()
		out := env.MakeApp(app, 1, true)
		route = testenv.AppRouteFromOutput(out)
		Expect(route).ToNot(BeEmpty())
	})

	AfterEach(func() {
		env.DeleteApp(app)
		env.DeleteNamespace(namespace)
	})

	readLogs := func(namespace, app string) string {
		token, err := authToken()
		Expect(err).ToNot(HaveOccurred())

		var urlArgs = []string{}
		urlArgs = append(urlArgs, fmt.Sprintf("follow=%t", false))
		wsURL := fmt.Sprintf("%s%s/%s?%s", websocketURL, v1.WsRoot, v1.WsRoutes.Path("AppLogs", namespace, app), strings.Join(urlArgs, "&"))
		wsConn, err := env.MakeWebSocketConnection(token, wsURL)
		Expect(err).ToNot(HaveOccurred())

		By("read the logs")
		var logs string
		// Allow up to 120s for log stream; slow environments may delay before close
		Eventually(func() bool {
			_, message, err := wsConn.ReadMessage()
			logLength++
			logs = fmt.Sprintf("%s %s", logs, string(message))
			return websocket.IsCloseError(err, websocket.CloseNormalClosure)
		}, 120*time.Second, 1*time.Second).Should(BeTrue())

		err = wsConn.Close()
		// With regular `ws` we could expect to not see any errors. With `wss`
		// however, with a tls layer in the mix, we can expect to see a `broken
		// pipe` issued. That is not a thing to act on, and is ignored.
		if err != nil && strings.Contains(err.Error(), "broken pipe") {
			return logs
		}
		Expect(err).ToNot(HaveOccurred())

		return logs
	}

	It("should send the logs", func() {
		logs := readLogs(namespace, app)

		By("checking if the logs are right")
		podNames := env.GetPodNames(app, namespace)
		for _, podName := range podNames {
			Expect(logs).To(ContainSubstring(podName))
		}
	})

	It("should follow logs", func() {
		existingLogs := readLogs(namespace, app)
		logLength := len(strings.Split(existingLogs, "\n"))

		token, err := authToken()
		Expect(err).ToNot(HaveOccurred(), "authToken failed")

		var urlArgs = []string{}
		urlArgs = append(urlArgs, fmt.Sprintf("follow=%t", true))
		wsURL := fmt.Sprintf("%s%s/%s?%s", websocketURL, v1.WsRoot, v1.WsRoutes.Path("AppLogs", namespace, app), strings.Join(urlArgs, "&"))
		wsConn, err := env.MakeWebSocketConnection(token, wsURL)
		Expect(err).ToNot(HaveOccurred(), "MakeWebSocketConnection failed for AppLogs")

		By("get to the end of logs")
		for i := 0; i < logLength; i++ {
			_, message, err := wsConn.ReadMessage()
			Expect(err).NotTo(HaveOccurred(), "AppLogs ReadMessage at index %d (logLength=%d): %v", i, logLength, err)
			Expect(message).NotTo(BeNil(), "AppLogs message nil at index %d", i)
		}

		By("adding more logs")
		routeHit := false
		deadline := time.Now().Add(3 * time.Minute)
		for time.Now().Before(deadline) && !routeHit {
			resp, err := env.Curl("GET", testenv.AppRouteWithPort(route), strings.NewReader(""))
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "[AppLogs] curl route failed (transient): %v\n", err)
				time.Sleep(3 * time.Second)
				continue
			}

			func() {
				defer resp.Body.Close()

				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "[AppLogs] read body failed (transient): %v\n", err)
					return
				}

				// reply must be from the phpinfo app
				if !strings.Contains(string(bodyBytes), "phpinfo()") {
					return
				}

				routeHit = resp.StatusCode == http.StatusOK
			}()
			if !routeHit {
				time.Sleep(3 * time.Second)
			}
		}
		if !routeHit {
			Fail("follow-log assertion failed: app route did not become reachable after retries")
		}

		By("checking the latest log message")
		var lastMessage string
		Eventually(func() string {
			_, message, err := wsConn.ReadMessage()
			if err != nil || message == nil {
				return ""
			}
			lastMessage = string(message)
			return lastMessage
		}, "3m", "3s").Should(ContainSubstring("[200]: GET /"), "expected log line [200]: GET / in websocket message; got: %q", lastMessage)

		err = wsConn.Close()
		Expect(err).ToNot(HaveOccurred())
	})
})
