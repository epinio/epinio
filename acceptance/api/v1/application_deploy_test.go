package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/helpers"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppDeploy Endpoint", func() {
	var (
		namespace string
		url       string
		body      string
		appName   string
		request   models.DeployRequest
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
