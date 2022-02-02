package v1_test

import (
	"fmt"
	"regexp"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/portforward"

	// api "github.com/epinio/epinio/internal/api/v1"
	// "github.com/epinio/epinio/pkg/api/core/v1/client"
	// "k8s.io/apimachinery/pkg/util/httpstream/spdy"
	// "k8s.io/client-go/transport"
	// gospdy "k8s.io/client-go/transport/spdy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("AppPortForward Endpoint", func() {
	var (
		appName   string
		namespace string
	)

	containerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()
		env.MakeContainerImageApp(appName, 1, containerImageURL)
	})
	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	Describe("GET /namespaces/:namespace/applications/:app/portforward", func() {
		var wsConn *websocket.Conn

		// BeforeEach(func() {
		// 	endpoint := fmt.Sprintf("%s%s/%s", c.URL, api.Root, api.Routes.Path("AppPortForward", namespace, appName))
		// 	portForwardURL, err := url.Parse(endpoint)
		// 	if err != nil {
		// 		return err
		// 	}

		// 	upgradeRoundTripper := client.NewUpgrader(spdy.RoundTripperConfig{
		// 		TLS:                      http.DefaultTransport.(*http.Transport).TLSClientConfig, // See `ExtendLocalTrust`
		// 		FollowRedirects:          true,
		// 		RequireSameHostRedirects: false,
		// 		PingPeriod:               time.Second * 5,
		// 	})

		// 	wrapper := transport.NewBasicAuthRoundTripper("admin", "password", upgradeRoundTripper)

		// 	dialer := gospdy.NewDialer(upgradeRoundTripper, &http.Client{Transport: wrapper}, "GET", portForwardURL)
		// })

		BeforeEach(func() {
			token, err := authToken()
			Expect(err).ToNot(HaveOccurred())
			wsURL := fmt.Sprintf("%s%s/%s", websocketURL, v1.WsRoot, v1.WsRoutes.Path("AppPortForward", namespace, appName))
			wsURL += "?port=8080"

			wsConn = env.MakeWebSocketConnection(token, wsURL, portforward.PortForwardProtocolV1Name)
			wsConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			wsConn.SetReadDeadline(time.Now().Add(10 * time.Second))
		})

		AfterEach(func() {
			// Ignore error, the connection will be already closed if the tests succeeds
			wsConn.Close()
		})

		It("runs a command and gets the output back", func() {
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
			err = wsConn.WriteMessage(websocket.TextMessage, command)
			Expect(err).ToNot(HaveOccurred())

			_, messageBytes, err = wsConn.ReadMessage()
			Expect(err).ToNot(HaveOccurred())

			// It prints command to stdout
			Expect(string(messageBytes)).To(ContainSubstring(cmdStr))

			// Exit the terminal
			cmdStr = "\nexit\n"
			command = append([]byte{0}, []byte(cmdStr)...)
			err = wsConn.WriteMessage(websocket.TextMessage, command)
			Expect(err).ToNot(HaveOccurred())

		})
	})
})
