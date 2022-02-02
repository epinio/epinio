package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/helmchart"
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

	When("staging the same app with no blob defined", func() {
		It("stages with the previous blob", func() {
			By("uploading the code")
			uploadResponse := uploadApplication(appName, namespace)
			oldBlob := uploadResponse.BlobUID

			stageRequest := models.StageRequest{
				App:          models.AppRef{Name: appName, Namespace: namespace},
				BlobUID:      oldBlob,
				BuilderImage: "paketobuildpacks/builder:full",
			}
			By("staging the application")
			stageResponse := stageApplication(appName, namespace, stageRequest)
			Eventually(listS3Blobs, "1m").Should(ContainElement(ContainSubstring(oldBlob)))

			stagingBlobUID, err := proc.Kubectl("get", "Jobs",
				"--namespace", helmchart.StagingNamespace,
				"-l", fmt.Sprintf("epinio.suse.org/stage-id=%s", stageResponse.Stage.ID),
				"-o", "jsonpath={.items[*].metadata.labels['epinio\\.suse\\.org/blob-uid']}")
			Expect(err).NotTo(HaveOccurred(), stagingBlobUID)
			Expect(stagingBlobUID).To(Equal(oldBlob))

			stageRequest = models.StageRequest{
				App:          models.AppRef{Name: appName, Namespace: namespace},
				BuilderImage: "paketobuildpacks/builder:full",
			}
			By("staging the application again")
			stageResponse = stageApplication(appName, namespace, stageRequest)

			stagingBlobUID, err = proc.Kubectl("get", "Jobs",
				"--namespace", helmchart.StagingNamespace,
				"-l", fmt.Sprintf("epinio.suse.org/stage-id=%s", stageResponse.Stage.ID),
				"-o", "jsonpath={.items[*].metadata.labels['epinio\\.suse\\.org/blob-uid']}")
			Expect(err).NotTo(HaveOccurred(), stagingBlobUID)
			Expect(stagingBlobUID).To(Equal(oldBlob))
		})
	})

	When("staging the same app with no BuilderImage defined", func() {
		It("stages with the previous builder image", func() {
			By("uploading the code")
			uploadResponse := uploadApplication(appName, namespace)
			oldBlob := uploadResponse.BlobUID
			builderImage := "paketobuildpacks/builder:full"

			stageRequest := models.StageRequest{
				App:          models.AppRef{Name: appName, Namespace: namespace},
				BlobUID:      oldBlob,
				BuilderImage: builderImage,
			}
			By("staging the application")
			stageResponse := stageApplication(appName, namespace, stageRequest)

			stagingBuilderImage, err := proc.Kubectl("get", "Pods",
				"--namespace", helmchart.StagingNamespace,
				"-l", fmt.Sprintf("epinio.suse.org/stage-id=%s", stageResponse.Stage.ID),
				"-o", "jsonpath={.items[*].spec.containers[0].image}")
			Expect(err).NotTo(HaveOccurred(), stagingBuilderImage)
			Expect(stagingBuilderImage).To(Equal(builderImage))

			stageRequest = models.StageRequest{
				App: models.AppRef{Name: appName, Namespace: namespace},
			}
			By("staging the application again")
			stageResponse = stageApplication(appName, namespace, stageRequest)

			stagingBuilderImage, err = proc.Kubectl("get", "Pods",
				"--namespace", helmchart.StagingNamespace,
				"-l", fmt.Sprintf("epinio.suse.org/stage-id=%s", stageResponse.Stage.ID),
				"-o", "jsonpath={.items[*].spec.containers[0].image}")
			Expect(err).NotTo(HaveOccurred(), stagingBuilderImage)
			Expect(stagingBuilderImage).To(Equal(builderImage))
		})
	})
	When("staging and deploying a new app", func() {
		It("returns a success", func() {
			By("uploading the code")
			uploadResponse := uploadApplication(appName, namespace)

			stageRequest := models.StageRequest{
				App:          models.AppRef{Name: appName, Namespace: namespace},
				BlobUID:      uploadResponse.BlobUID,
				BuilderImage: "paketobuildpacks/builder:full",
			}

			By("staging the application")
			stageResponse := stageApplication(appName, namespace, stageRequest)

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
