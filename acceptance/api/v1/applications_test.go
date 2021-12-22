package v1_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/helpers"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps API Application Endpoints", func() {
	var (
		namespace string
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		// Wait for server to be up and running
		Eventually(func() error {
			_, err := env.Curl("GET", serverURL+v1.Root+"/info", strings.NewReader(""))
			return err
		}, "1m").ShouldNot(HaveOccurred())
	})
	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	Context("Logs", func() {
		Describe("GET /api/v1/namespaces/:namespaces/applications/:app/logs", func() {
			logLength := 0
			var (
				route string
				app   string
			)

			BeforeEach(func() {
				app = catalog.NewAppName()
				out := env.MakeApp(app, 1, true)
				route = testenv.AppRouteFromOutput(out)
				Expect(route).ToNot(BeEmpty())
			})

			AfterEach(func() {
				env.DeleteApp(app)
			})

			readLogs := func(namespace, app string) string {
				var urlArgs = []string{}
				urlArgs = append(urlArgs, fmt.Sprintf("follow=%t", false))
				wsURL := fmt.Sprintf("%s%s/%s?%s", websocketURL, v1.Root, v1.Routes.Path("AppLogs", namespace, app), strings.Join(urlArgs, "&"))
				wsConn := env.MakeWebSocketConnection(wsURL)

				By("read the logs")
				var logs string
				Eventually(func() bool {
					_, message, err := wsConn.ReadMessage()
					logLength++
					logs = fmt.Sprintf("%s %s", logs, string(message))
					return websocket.IsCloseError(err, websocket.CloseNormalClosure)
				}, 30*time.Second, 1*time.Second).Should(BeTrue())

				err := wsConn.Close()
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

				var urlArgs = []string{}
				urlArgs = append(urlArgs, fmt.Sprintf("follow=%t", true))
				wsURL := fmt.Sprintf("%s%s/%s?%s", websocketURL, v1.Root, v1.Routes.Path("AppLogs", namespace, app), strings.Join(urlArgs, "&"))
				wsConn := env.MakeWebSocketConnection(wsURL)

				By("get to the end of logs")
				for i := 0; i < logLength-1; i++ {
					_, message, err := wsConn.ReadMessage()
					Expect(err).NotTo(HaveOccurred())
					Expect(message).NotTo(BeNil())
				}

				By("adding more logs")
				Eventually(func() int {
					resp, err := env.Curl("GET", route, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())

					defer resp.Body.Close()

					bodyBytes, err := ioutil.ReadAll(resp.Body)
					Expect(err).ToNot(HaveOccurred(), resp)

					// reply must be from the phpinfo app
					if !strings.Contains(string(bodyBytes), "phpinfo()") {
						return 0
					}

					return resp.StatusCode
				}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

				By("checking the latest log message")
				Eventually(func() string {
					_, message, err := wsConn.ReadMessage()
					Expect(err).NotTo(HaveOccurred())
					Expect(message).NotTo(BeNil())
					return string(message)
				}, "10s").Should(ContainSubstring("GET / HTTP/1.1"))

				err := wsConn.Close()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("Creating", func() {
		var (
			appName string
		)

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)
			appName = catalog.NewAppName()
		})

		AfterEach(func() {
			Eventually(func() string {
				out, err := env.Epinio("", "app", "delete", appName)
				if err != nil {
					return out
				}
				return ""
			}, "5m").Should(BeEmpty())
		})

		When("creating a new app", func() {
			It("creates the app resource", func() {
				response, err := createApplication(appName, namespace, []string{"mytestdomain.org"})
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()

				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
				out, err := helpers.Kubectl("get", "apps", "-n", namespace, appName, "-o", "jsonpath={.spec.routes[*]}")
				Expect(err).ToNot(HaveOccurred(), out)
				routes := strings.Split(out, " ")
				Expect(len(routes)).To(Equal(1))
				Expect(routes[0]).To(Equal("mytestdomain.org"))
			})
		})
	})
})
