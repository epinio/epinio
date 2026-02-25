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
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppPortForward Endpoint", LApplication, func() {
	var (
		appName   string
		namespace string
	)

	containerImageURL := "epinio/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()
		env.MakeContainerImageApp(appName, 1, containerImageURL)
	})

	AfterEach(func() {
		env.DeleteApp(appName)
		env.DeleteNamespace(namespace)
	})

	Describe("GET /namespaces/:namespace/applications/:app/portforward", func() {

		When("you don't specify an instance", func() {
			It("runs a GET through the opened stream and gets the response back", func() {
				// Keep this resilient for transient websocket/SPDY setup issues seen in CI.
				var lastErr error
				Eventually(func() error {
					lastErr = runPortForwardGet(namespace, appName, "")
					return lastErr
				}, "10m", "20s").Should(Succeed(), "AppPortForward GET (no instance) failed for namespace=%s app=%s: %v", namespace, appName, lastErr)
			})
		})

		When("you specify a non existing instance", func() {
			var connErr error

			BeforeEach(func() {
				_, connErr = setupConnection(namespace, appName, "nonexisting")
			})

			It("fails with a 400 bad request", func() {
				if connErr == nil {
					fmt.Fprintf(GinkgoWriter, "[AppPortForward] expected connection to nonexisting instance to fail; got nil error\n")
				} else {
					fmt.Fprintf(GinkgoWriter, "[AppPortForward] connection to nonexisting instance failed as expected: %v\n", connErr)
				}
				Expect(connErr).To(HaveOccurred(), "expected dial to nonexisting instance to fail")
			})
		})

		When("you specify a specific instance", func() {
			var appName string
			var instanceName string

			BeforeEach(func() {
				// Bug fix: Use separate application instead of the main of the suite
				appName = "portforward"

				env.MakeContainerImageApp(appName, 2, containerImageURL)

				out, err := proc.Kubectl("get", "pods",
					"-n", namespace,
					"-l", fmt.Sprintf("app.kubernetes.io/name=%s", appName),
					"-o", "name",
				)
				Expect(err).ToNot(HaveOccurred())

				podNames := strings.Split(strings.TrimSpace(out), "\n")
				Expect(len(podNames)).To(Equal(2))

				instanceName = strings.Replace(podNames[1], "pod/", "", -1)
			})

			AfterEach(func() {
				env.DeleteApp(appName)
			})

			It("runs a GET through the opened stream and gets the response back", func() {
				// Keep this resilient for transient websocket/SPDY setup issues seen in CI.
				var lastErr error
				Eventually(func() error {
					lastErr = runPortForwardGet(namespace, appName, instanceName)
					return lastErr
				}, "10m", "20s").Should(Succeed(), "AppPortForward GET (instance=%s) failed for namespace=%s app=%s: %v", instanceName, namespace, appName, lastErr)
			})
		})
	})
})

func runPortForwardGet(namespace, appName, instance string) error {
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		lastErr = runPortForwardGetOnce(namespace, appName, instance, attempt)
		if lastErr == nil {
			return nil
		}
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	return fmt.Errorf("port-forward GET failed after retries: %w", lastErr)
}

func runPortForwardGetOnce(namespace, appName, instance string, attempt int) error {
	start := time.Now()
	fmt.Fprintf(GinkgoWriter, "[AppPortForward] attempt=%d start namespace=%s app=%s instance=%q\n", attempt, namespace, appName, instance)

	conn, err := setupConnection(namespace, appName, instance)
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "[AppPortForward] setupConnection failed after %v: %v\n", time.Since(start), err)
		return fmt.Errorf("setupConnection: %w", err)
	}
	defer conn.Close()

	stream := conn.UnderlyingConn()

	// Let the port-forward stream stabilize before sending (reduces EOF under load).
	time.Sleep(3 * time.Second)

	// Send a GET request through the stream.
	req, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/", nil)
	if err = req.Write(stream); err != nil {
		fmt.Fprintf(GinkgoWriter, "[AppPortForward] req.Write failed after %v: %v\n", time.Since(start), err)
		return fmt.Errorf("req.Write: %w", err)
	}

	fmt.Fprintf(GinkgoWriter, "[AppPortForward] ReadResponse attempt (elapsed %v) namespace=%s app=%s\n", time.Since(start), namespace, appName)
	reader := bufio.NewReader(stream)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "[AppPortForward] ReadResponse failed after %v (often EOF under load): %v\n", time.Since(start), err)
		fmt.Fprintf(GinkgoWriter, "[AppPortForward] root cause: stream closed before HTTP response - server may have closed connection or is under load\n")
		return fmt.Errorf("ReadResponse: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(GinkgoWriter, "[AppPortForward] unexpected status code: got %d (expected 200)\n", resp.StatusCode)
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func setupConnection(namespace, appName, instance string) (*websocket.Conn, error) {
	endpoint := fmt.Sprintf("%s%s/%s?instance=%s", serverURL, api.WsRoot, api.WsRoutes.Path("AppPortForward", namespace, appName), instance)
	portForwardURL, err := url.Parse(endpoint)
	Expect(err).ToNot(HaveOccurred())

	token, err := authToken()
	Expect(err).ToNot(HaveOccurred())

	values := portForwardURL.Query()
	values.Add("authtoken", token)
	portForwardURL.RawQuery = values.Encode()
	portForwardURL.Scheme = "wss"

	conn, _, err := websocket.DefaultDialer.Dial(portForwardURL.String(), nil)

	return conn, err
}
