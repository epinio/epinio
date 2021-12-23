package v1_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/testenv"
	v1 "github.com/epinio/epinio/internal/api/v1"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppStage Endpoint", func() {
	var (
		url       string
		body      string
		appName   string
		request   models.DeployRequest
		namespace string
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
		env.DeleteNamespace(namespace)
	})

	When("staging an app with the blob of a different app", func() {
		var appName2 string
		var uploadResponse2 *models.UploadResponse

		BeforeEach(func() {
			appName2 = catalog.NewAppName()

			By("creating the other application resource first")
			_, err := createApplication(appName2, namespace, []string{})
			Expect(err).ToNot(HaveOccurred())

			By("uploading the code of the other")
			uploadResponse2 = uploadApplication(appName2, namespace)

			By("uploading the code of itself")
			_ = uploadApplication(appName, namespace)
		})

		AfterEach(func() {
			env.DeleteApp(appName2)
		})

		It("fails to stage", func() {
			// Inlined stageApplication() to check for the error.
			// Note how appName and uploadResponse2 are mixed.

			request := models.StageRequest{
				App: models.AppRef{
					Name:      appName, // App 1
					Namespace: namespace,
				},
				BlobUID:      uploadResponse2.BlobUID, // Code 2
				BuilderImage: "paketobuildpacks/builder:full",
			}
			b, err := json.Marshal(request)
			Expect(err).NotTo(HaveOccurred())
			body := string(b)

			url := serverURL + v1.Root + "/" + v1.Routes.Path("AppStage", namespace, appName)
			response, err := env.Curl("POST", url, strings.NewReader(body))
			Expect(err).NotTo(HaveOccurred())

			b, err = ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(b))

			errResponse := &apierrors.ErrorResponse{}
			err = json.Unmarshal(b, errResponse)
			Expect(err).NotTo(HaveOccurred())

			Expect(errResponse.Errors).To(HaveLen(1))
			Expect(errResponse.Errors[0].Title).To(Equal("blob app mismatch"))
			Expect(errResponse.Errors[0].Details).To(Equal("expected: " + appName + ", found: " + appName2))
		})
	})

	When("staging the same app with a new blob", func() {
		It("cleans up old S3 objects", func() {
			By("uploading the code")
			uploadResponse := uploadApplication(appName, namespace)
			oldBlob := uploadResponse.BlobUID
			By("staging the application")
			_ = stageApplication(appName, namespace, uploadResponse)
			Eventually(listS3Blobs, "1m").Should(ContainElement(ContainSubstring(oldBlob)))

			By("uploading the code again")
			uploadResponse = uploadApplication(appName, namespace)
			newBlob := uploadResponse.BlobUID
			By("staging the application again")
			_ = stageApplication(appName, namespace, uploadResponse)

			Eventually(listS3Blobs, "2m").Should(ContainElement(ContainSubstring(newBlob)))
			Eventually(listS3Blobs, "2m").ShouldNot(ContainElement(ContainSubstring(oldBlob)))
		})
	})

	When("staging and deploying a new app", func() {
		It("returns a success", func() {
			By("uploading the code")
			uploadResponse := uploadApplication(appName, namespace)

			By("staging the application")
			stageResponse := stageApplication(appName, namespace, uploadResponse)

			By("deploying the staged resource")
			request = models.DeployRequest{
				App: models.AppRef{
					Name:      appName,
					Namespace: namespace,
				},
				Stage: models.StageRef{
					ID: stageResponse.Stage.ID,
				},
				ImageURL: stageResponse.ImageURL,
				Origin: models.ApplicationOrigin{
					Kind: models.OriginPath,
					Path: testenv.TestAssetPath("sample-app.tar"),
				},
			}

			bodyBytes, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())
			body = string(bodyBytes)

			url = serverURL + v1.Root + "/" + v1.Routes.Path("AppDeploy", namespace, appName)

			response, err := env.Curl("POST", url, strings.NewReader(body))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()

			bodyBytes, err = ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			deploy := &models.DeployResponse{}
			err = json.Unmarshal(bodyBytes, deploy)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy.Routes[0]).To(MatchRegexp(appName + `.*\.omg\.howdoi\.website`))

			By("waiting for the deployment to complete")

			url = serverURL + v1.Root + "/" + v1.Routes.Path("AppRunning", namespace, appName)

			response, err = env.Curl("GET", url, strings.NewReader(body))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()

			By("confirming at highlevel")
			// Highlevel check and confirmation
			Eventually(func() string {
				return appFromAPI(namespace, appName).Workload.Status
			}, "5m").Should(Equal("1/1"))
		})
	})
})
