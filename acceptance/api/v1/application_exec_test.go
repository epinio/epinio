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
	"regexp"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/gorilla/websocket"
	"k8s.io/apimachinery/pkg/util/httpstream/wsstream"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppExec Endpoint", LApplication, func() {
	var (
		appName   string
		namespace string
	)

	containerImageURL := "epinio/sample-app"

	Describe("GET /namespaces/:namespace/applications/:app/exec", func() {
		var wsConn *websocket.Conn
		var wsURL string

		When("no instance is specified", func() {
			BeforeEach(func() { // We need to set the wsURL before we run this
				namespace = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespace)
				appName = catalog.NewAppName()
				env.MakeContainerImageApp(appName, 1, containerImageURL)

				wsURL = fmt.Sprintf("%s%s/%s", websocketURL, v1.WsRoot, v1.WsRoutes.Path("AppExec", namespace, appName))
				token, err := authToken()
				Expect(err).ToNot(HaveOccurred())

				// Beware! When the "raw" protocol is used (wsstream.ChannelWebSocketProtocol)
				// the channel is defined by the first byte.
				// In the wsstream.Base64ChannelWebSocketProtocol case, the first byte
				// is considered to be the ascii code of the channel. E.g. byte 48 for "0"
				// https://github.com/kubernetes/kubernetes/blob/46c5edbc58b81046ce799875dc611beaaf0ffb44/staging/src/k8s.io/apiserver/pkg/util/wsstream/conn.go#L261-L264
				// base64: append([]byte("0"), []byte(base64.URLEncoding.EncodeToString([]byte(cmdStr)))...)
				//    raw: append([]byte{0}, []byte(cmdStr)...)
				wsConn, err = env.MakeWebSocketConnection(token, wsURL, wsstream.ChannelWebSocketProtocol)
				Expect(err).ToNot(HaveOccurred())
				wsConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				wsConn.SetReadDeadline(time.Now().Add(10 * time.Second))
			})

			AfterEach(func() {
				// Ignore error, the connection will be already closed if the tests succeeds
				wsConn.Close()
				env.DeleteNamespace(namespace)
			})

			It("runs a command and gets the output back", func() {
				// Run command: echo "test" > /workspace/test && exit
				// Check stdout stream (it should send back the command we sent)
				// Check if the file exists on the application Pod with kubectl

				var messageBytes []byte
				var err error

				// Read until we reach the prompt
				r, err := regexp.Compile(`.*\$`) // Matches the bash command prompt
				Expect(err).ToNot(HaveOccurred())
				for !r.MatchString(string(messageBytes)) {
					_, newBytes, err := wsConn.ReadMessage()
					Expect(err).ToNot(HaveOccurred())
					messageBytes = append(messageBytes, newBytes[1:]...) // Skip the "channel" byte
				}

				// Run the command
				cmdStr := "echo testing-epinio > /workspace/test-echo"
				command := append([]byte{0}, []byte(cmdStr)...)
				err = wsConn.WriteMessage(websocket.BinaryMessage, command)
				Expect(err).ToNot(HaveOccurred())

				messageBytes = []byte{} // Empty the slice
				// When we hit the "end of the terminal width" we will get a carriage return
				// along with the last character again. Resizing the terminal is complicated
				// and we avoid this here with this trick. The number of characters we can
				// write before we get a carriage return (ascii 13) depends on the size
				// of the prompt, which in turn depends on the name of the Pod.
				replaceRegex := regexp.MustCompile(`.\r`)
				Eventually(func() string {
					_, newBytes, err := wsConn.ReadMessage()
					Expect(err).ToNot(HaveOccurred())

					messageBytes = append(messageBytes, newBytes[1:]...) // Skip the "channel" byte
					return replaceRegex.ReplaceAllString(string(messageBytes), "")
				}, "20s", "1s").Should(MatchRegexp(cmdStr))

				// Exit the terminal
				cmdStr = "\nexit\n"
				command = append([]byte{0}, []byte(cmdStr)...)
				err = wsConn.WriteMessage(websocket.TextMessage, command)
				Expect(err).ToNot(HaveOccurred())

				// Check the effects of the command we run
				out, err := proc.Kubectl("get", "pods",
					"-l", fmt.Sprintf("app.kubernetes.io/name=%s", appName),
					"-n", namespace, "-o", "name")
				Expect(err).ToNot(HaveOccurred())

				out, err = proc.Kubectl("exec",
					strings.TrimSpace(out), "-n", namespace,
					"--", "cat", "/workspace/test-echo")
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(strings.TrimSpace(out)).To(Equal("testing-epinio"))
			})
		})

		When("the specified instance does not exist", func() {
			BeforeEach(func() {
				namespace = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespace)
				appName = catalog.NewAppName()
				env.MakeContainerImageApp(appName, 2, containerImageURL)
			})

			AfterEach(func() {
				env.DeleteNamespace(namespace)
			})

			It("returns an error", func() {
				wsURL = fmt.Sprintf("%s%s/%s?instance=doesnotexist", websocketURL, v1.WsRoot, v1.WsRoutes.Path("AppExec", namespace, appName))
				token, err := authToken()
				Expect(err).ToNot(HaveOccurred())
				wsConn, err = env.MakeWebSocketConnection(token, wsURL, wsstream.ChannelWebSocketProtocol)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(MatchRegexp("specified instance doesn't exist"))
			})
		})

		When("the specified instance exists", func() {
			BeforeEach(func() {
				namespace = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespace)
				appName = catalog.NewAppName()
				env.MakeContainerImageApp(appName, 2, containerImageURL)

				out, err := proc.Kubectl("get", "pods",
					"-n", namespace,
					"-l", fmt.Sprintf("app.kubernetes.io/part-of=%s,app.kubernetes.io/name=%s", namespace, appName),
					"-o", "name",
				)
				Expect(err).ToNot(HaveOccurred())

				podNames := strings.Split(strings.TrimSpace(out), "\n")
				Expect(len(podNames)).To(Equal(2))

				podToExec := strings.Replace(podNames[1], "pod/", "", -1)
				wsURL = fmt.Sprintf("%s%s/%s?instance=%s", websocketURL, v1.WsRoot, v1.WsRoutes.Path("AppExec", namespace, appName), podToExec)
			})

			AfterEach(func() {
				env.DeleteNamespace(namespace)
			})

			It("works", func() {
				token, err := authToken()
				Expect(err).ToNot(HaveOccurred())
				wsConn, err = env.MakeWebSocketConnection(token, wsURL, wsstream.ChannelWebSocketProtocol)
				Expect(err).ToNot(HaveOccurred())
				wsConn.Close()
			})
		})
	})
})
