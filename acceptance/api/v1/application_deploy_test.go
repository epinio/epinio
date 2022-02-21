package v1_test

import (
	"fmt"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppDeploy Endpoint", func() {
	var (
		namespace string
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

	Context("with staging", func() {
		var deployRequest models.DeployRequest

		BeforeEach(func() {
			deployRequest = models.DeployRequest{
				App: models.AppRef{
					Name:      appName,
					Namespace: namespace,
				},
				Origin: models.ApplicationOrigin{
					Kind: models.OriginPath,
					Path: testenv.TestAssetPath("sample-app.tar"),
				},
			}
		})

		When("staging, no other job for the same blob exists", func() {
			It("cleans up old S3 objects", func() {
				By("uploading the code")
				uploadResponse := uploadApplication(appName, namespace)
				oldBlob := uploadResponse.BlobUID

				stageRequest := models.StageRequest{
					App:          models.AppRef{Name: appName, Namespace: namespace},
					BlobUID:      oldBlob,
					BuilderImage: "paketobuildpacks/builder:full",
				}
				By("staging the application")
				_ = stageApplication(appName, namespace, stageRequest)
				Eventually(listS3Blobs, "1m").Should(ContainElement(ContainSubstring(oldBlob)))

				By("uploading the code again")
				uploadResponse = uploadApplication(appName, namespace)
				newBlob := uploadResponse.BlobUID

				stageRequest = models.StageRequest{
					App:          models.AppRef{Name: appName, Namespace: namespace},
					BlobUID:      newBlob,
					BuilderImage: "paketobuildpacks/builder:full",
				}
				By("staging the application again")
				stageResponse := stageApplication(appName, namespace, stageRequest)

				Eventually(listS3Blobs, "2m").Should(ContainElement(ContainSubstring(newBlob)))

				deployRequest.ImageURL = stageResponse.ImageURL
				deployRequest.Stage.ID = stageRequest.BlobUID
				deployApplication(appName, namespace, deployRequest)

				Eventually(listS3Blobs, "2m").ShouldNot(ContainElement(ContainSubstring(oldBlob)))
			})
		})

		When("an older staging job for the same blob exists", func() {
			It("doesn't delete the S3 object", func() {
				By("uploading the code")
				uploadResponse := uploadApplication(appName, namespace)
				theOnlyBlob := uploadResponse.BlobUID

				stageRequest := models.StageRequest{
					App:          models.AppRef{Name: appName, Namespace: namespace},
					BlobUID:      theOnlyBlob,
					BuilderImage: "paketobuildpacks/builder:full",
				}

				By("staging the application")
				_ = stageApplication(appName, namespace, stageRequest)
				Eventually(listS3Blobs, "1m").Should(ContainElement(ContainSubstring(theOnlyBlob)))

				By("staging the application again")
				stageResponse := stageApplication(appName, namespace, stageRequest)

				// sanity check
				out, err := proc.Kubectl("get", "Jobs",
					"--namespace", helmchart.StagingNamespace,
					"-o", "jsonpath={.items[*].metadata.labels['epinio\\.suse\\.org/blob-uid']}")
				Expect(err).NotTo(HaveOccurred(), out)
				blobUIDs := strings.Split(out, " ")
				count := 0
				for _, b := range blobUIDs {
					if b == theOnlyBlob {
						count += 1
					}
				}
				Expect(count).To(Equal(2))

				deployRequest.ImageURL = stageResponse.ImageURL
				deployRequest.Stage.ID = stageResponse.Stage.ID
				deployApplication(appName, namespace, deployRequest)

				Consistently(listS3Blobs, "2m").Should(ContainElement(ContainSubstring(theOnlyBlob)))
			})
		})
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
		})

		When("deploying a new app", func() {
			It("returns a success", func() {
				deployResponse := deployApplication(appName, namespace, request)

				Expect(deployResponse.Routes[0]).To(MatchRegexp(appName + `.*\.omg\.howdoi\.website`))

				Eventually(func() string {
					return appFromAPI(namespace, appName).Workload.Status
				}, "5m").Should(Equal("1/1"))

				// Check if autoserviceaccounttoken is true
				labels := fmt.Sprintf("app.kubernetes.io/name=%s", appName)
				out, err := proc.Kubectl("get", "pod",
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
				routes = []string{"appdomain.org", "appdomain2.org"}
				out, err := proc.Kubectl("patch", "apps", "--type", "json",
					"-n", namespace, appName, "--patch",
					fmt.Sprintf(`[{"op": "replace", "path": "/spec/routes", "value": [%q, %q]}]`, routes[0], routes[1]))
				Expect(err).NotTo(HaveOccurred(), out)
			})

			It("the app Ingress matches the specified route", func() {
				// call the deploy action. Deploy should respect the routes on the App CR.
				deployApplication(appName, namespace, request)

				out, err := proc.Kubectl("get", "ingress",
					"--namespace", namespace, "-o", "jsonpath={.items[*].spec.rules[0].host}")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(strings.Split(out, " ")).To(Equal(routes))
			})
		})
	})
})
