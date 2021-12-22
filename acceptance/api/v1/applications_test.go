package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/helpers"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
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

	Context("Uploading", func() {

		var (
			url     string
			path    string
			request *http.Request
		)

		JustBeforeEach(func() {
			url = serverURL + v1.Root + "/" + v1.Routes.Path("AppUpload", namespace, "testapp")
			var err error
			request, err = uploadRequest(url, path)
			Expect(err).ToNot(HaveOccurred())
		})

		When("uploading a new dir", func() {
			BeforeEach(func() {
				path = testenv.TestAssetPath("sample-app.tar")
			})

			It("returns the app response", func() {
				resp, err := env.Client().Do(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				defer resp.Body.Close()

				bodyBytes, err := ioutil.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				r := &models.UploadResponse{}
				err = json.Unmarshal(bodyBytes, &r)
				Expect(err).ToNot(HaveOccurred())

				Expect(r.BlobUID).ToNot(BeEmpty())
			})
		})
	})

	Context("Deploying", func() {
		var (
			url     string
			body    string
			appName string
			request models.DeployRequest
		)

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)
			appName = catalog.NewAppName()

			By("creating application resource first")
			_, err := createApplication(appName, namespace, []string{})
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			env.DeleteApp(appName)
		})

		Context("with non-staging using custom container image", func() {
			BeforeEach(func() {
				request = models.DeployRequest{
					App: models.AppRef{
						Name:      appName,
						Namespace: namespace,
					},
					ImageURL: "splatform/sample-app",
					Origin: models.ApplicationOrigin{
						Kind:      models.OriginContainer,
						Container: "splatform/sample-app",
					},
				}

				url = serverURL + v1.Root + "/" + v1.Routes.Path("AppDeploy", namespace, appName)
			})

			When("deploying a new app", func() {
				BeforeEach(func() {
					bodyBytes, err := json.Marshal(request)
					Expect(err).ToNot(HaveOccurred())
					body = string(bodyBytes)
				})

				It("returns a success", func() {
					response, err := env.Curl("POST", url, strings.NewReader(body))
					Expect(err).ToNot(HaveOccurred())
					Expect(response).ToNot(BeNil())
					defer response.Body.Close()

					bodyBytes, err := ioutil.ReadAll(response.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

					deploy := &models.DeployResponse{}
					err = json.Unmarshal(bodyBytes, deploy)
					Expect(err).NotTo(HaveOccurred())
					Expect(deploy.Routes[0]).To(MatchRegexp(appName + `.*\.omg\.howdoi\.website`))

					Eventually(func() string {
						return appFromAPI(namespace, appName).Workload.Status
					}, "5m").Should(Equal("1/1"))

					// Check if autoserviceaccounttoken is true
					labels := fmt.Sprintf("app.kubernetes.io/name=%s", appName)
					out, err := helpers.Kubectl("get", "pod",
						"--namespace", namespace,
						"-l", labels,
						"-o", "jsonpath={.items[*].spec.automountServiceAccountToken}")
					Expect(err).NotTo(HaveOccurred(), out)
					Expect(out).To(ContainSubstring("true"))
				})
			})

			When("deploying an app with custom routes", func() {
				var routes []string
				BeforeEach(func() {
					routes = append(routes, "appdomain.org", "appdomain2.org")
					out, err := helpers.Kubectl("patch", "apps", "--type", "json",
						"-n", namespace, appName, "--patch",
						fmt.Sprintf(`[{"op": "replace", "path": "/spec/routes", "value": [%q, %q]}]`, routes[0], routes[1]))
					Expect(err).NotTo(HaveOccurred(), out)
				})

				It("the app Ingress matches the specified route", func() {
					bodyBytes, err := json.Marshal(request)
					Expect(err).ToNot(HaveOccurred())
					body = string(bodyBytes)
					// call the deploy action. Deploy should respect the routes on the App CR.
					_, err = env.Curl("POST", url, strings.NewReader(body))
					Expect(err).ToNot(HaveOccurred())

					out, err := helpers.Kubectl("get", "ingress",
						"--namespace", namespace, "-o", "jsonpath={.items[*].spec.rules[0].host}")
					Expect(err).NotTo(HaveOccurred(), out)
					Expect(strings.Split(out, " ")).To(Equal(routes))
				})
			})
		})
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
