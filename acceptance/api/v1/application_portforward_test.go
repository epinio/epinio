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
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/tools/portforward"
	gospdy "k8s.io/client-go/transport/spdy"

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
			var conn httpstream.Connection
			var connErr error

			BeforeEach(func() {
				conn, connErr = setupConnection(namespace, appName, "")
			})

			AfterEach(func() {
				conn.Close()
			})

			It("runs a GET through the opened stream and gets the response back", func() {
				Expect(connErr).ToNot(HaveOccurred())
				streamData, streamErr := createStreams(conn)

				// send a GET request through the stream
				req, _ := http.NewRequest(http.MethodGet, "http://localhost:8080", nil)
				Expect(req.Write(streamData)).ToNot(HaveOccurred())

				// read incoming data and parse the response
				resp, err := http.ReadResponse(bufio.NewReader(streamData), req)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// check that there are no errors in the error stream
				errData, err := io.ReadAll(streamErr)
				Expect(err).ToNot(HaveOccurred())
				Expect(errData).To(BeEmpty())

				// close streams
				Expect(streamData.Close()).ToNot(HaveOccurred())
				Expect(streamErr.Close()).ToNot(HaveOccurred())
			})
		})

		When("you specify a non existing instance", func() {
			var connErr error

			BeforeEach(func() {
				_, connErr = setupConnection(namespace, appName, "nonexisting")
			})

			It("fails with a 400 bad request", func() {
				Expect(connErr).To(HaveOccurred())
			})
		})

		When("you specify a specific instance", func() {
			var conn httpstream.Connection
			var connErr error
			var appName string

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

				selectedInstance := strings.Replace(podNames[1], "pod/", "", -1)
				conn, connErr = setupConnection(namespace, appName, selectedInstance)
			})

			AfterEach(func() {
				conn.Close()
				env.DeleteApp(appName)
			})

			It("runs a GET through the opened stream and gets the response back", func() {
				Expect(connErr).ToNot(HaveOccurred())
				streamData, streamErr := createStreams(conn)

				// send a GET request through the stream
				req, _ := http.NewRequest(http.MethodGet, "http://localhost:8080", nil)
				Expect(req.Write(streamData)).ToNot(HaveOccurred())

				// read incoming data and parse the response
				resp, err := http.ReadResponse(bufio.NewReader(streamData), req)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// check that there are no errors in the error stream
				errData, err := io.ReadAll(streamErr)
				Expect(err).ToNot(HaveOccurred())
				Expect(errData).To(BeEmpty())

				// close streams
				Expect(streamData.Close()).ToNot(HaveOccurred())
				Expect(streamErr.Close()).ToNot(HaveOccurred())
			})
		})
	})
})

func setupConnection(namespace, appName, instance string) (httpstream.Connection, error) {
	endpoint := fmt.Sprintf("%s%s/%s?instance=%s", serverURL, api.WsRoot, api.WsRoutes.Path("AppPortForward", namespace, appName), instance)
	portForwardURL, err := url.Parse(endpoint)
	Expect(err).ToNot(HaveOccurred())

	token, err := authToken()
	Expect(err).ToNot(HaveOccurred())

	values := portForwardURL.Query()
	values.Add("authtoken", token)
	portForwardURL.RawQuery = values.Encode()

	// we need to use the spdy client to handle this connection
	upgradeRoundTripper, err := client.NewUpgrader(spdy.RoundTripperConfig{
		TLS:        http.DefaultTransport.(*http.Transport).TLSClientConfig, // See `ExtendLocalTrust`
		PingPeriod: time.Second * 5,
	})
	Expect(err).ToNot(HaveOccurred())

	dialer := gospdy.NewDialer(upgradeRoundTripper, &http.Client{Transport: upgradeRoundTripper}, "GET", portForwardURL)
	conn, _, err := dialer.Dial(portforward.PortForwardProtocolV1Name)

	return conn, err
}

func createStreams(conn httpstream.Connection) (httpstream.Stream, httpstream.Stream) {
	buildHeaders := func(streamType string) http.Header {
		headers := http.Header{}
		headers.Set(v1.PortHeader, "8080")
		headers.Set(v1.PortForwardRequestIDHeader, "0")
		headers.Set(v1.StreamType, streamType)
		return headers
	}

	// open streams
	streamData, err := conn.CreateStream(buildHeaders(v1.StreamTypeData))
	Expect(err).ToNot(HaveOccurred())
	streamErr, err := conn.CreateStream(buildHeaders(v1.StreamTypeError))
	Expect(err).ToNot(HaveOccurred())

	return streamData, streamErr
}
