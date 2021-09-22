package v1_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/randstr"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps API Application Endpoints", func() {
	var (
		org string
		one int32 = 1
		two int32 = 2
	)
	dockerImageURL := "splatform/sample-app"

	uploadRequest := func(url, path string) (*http.Request, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, errors.Wrap(err, "failed to open tarball")
		}
		defer file.Close()

		// create multipart form
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", filepath.Base(file.Name()))
		if err != nil {
			return nil, errors.Wrap(err, "failed to create multiform part")
		}

		_, err = io.Copy(part, file)
		if err != nil {
			return nil, errors.Wrap(err, "failed to write to multiform part")
		}

		err = writer.Close()
		if err != nil {
			return nil, errors.Wrap(err, "failed to close multiform")
		}

		// make the request
		request, err := http.NewRequest("POST", url, body)
		request.SetBasicAuth(env.EpinioUser, env.EpinioPassword)
		if err != nil {
			return nil, errors.Wrap(err, "failed to build request")
		}
		request.Header.Add("Content-Type", writer.FormDataContentType())

		return request, nil
	}

	appStatus := func(org, app string) string {
		response, err := env.Curl("GET",
			fmt.Sprintf("%s/api/v1/namespaces/%s/applications/%s", serverURL, org, app),
			strings.NewReader(""))

		ExpectWithOffset(1, err).ToNot(HaveOccurred())
		ExpectWithOffset(1, response).ToNot(BeNil())
		defer response.Body.Close()
		ExpectWithOffset(1, response.StatusCode).To(Equal(http.StatusOK))
		bodyBytes, err := ioutil.ReadAll(response.Body)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())

		var responseApp models.App
		err = json.Unmarshal(bodyBytes, &responseApp)
		ExpectWithOffset(1, err).ToNot(HaveOccurred(), string(bodyBytes))
		ExpectWithOffset(1, responseApp.Name).To(Equal(app))
		ExpectWithOffset(1, responseApp.Organization).To(Equal(org))

		return responseApp.Status
	}

	updateAppInstances := func(org string, app string, instances int32) (int, []byte) {
		data, err := json.Marshal(models.UpdateAppRequest{Instances: instances})
		ExpectWithOffset(1, err).ToNot(HaveOccurred())

		response, err := env.Curl("PATCH",
			fmt.Sprintf("%s/api/v1/namespaces/%s/applications/%s", serverURL, org, app),
			strings.NewReader(string(data)))
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
		ExpectWithOffset(1, response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())

		return response.StatusCode, bodyBytes
	}

	createApplication := func(name string, org string) (*http.Response, error) {
		request := models.ApplicationCreateRequest{Name: name}
		b, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		body := string(b)

		url := serverURL + "/" + v1.Routes.Path("AppCreate", org)
		return env.Curl("POST", url, strings.NewReader(body))
	}

	waitForPipeline := func(stageID string) {
		Eventually(func() string {
			out, err := helpers.Kubectl("get", "pipelinerun",
				"--namespace", deployments.TektonStagingNamespace,
				stageID,
				"-o", "jsonpath={.status.conditions[0].status}")
			Expect(err).NotTo(HaveOccurred())
			return out
		}, "5m").Should(Equal("True"))
	}

	uploadApplication := func(appName string) *models.UploadResponse {
		uploadURL := serverURL + "/" + v1.Routes.Path("AppUpload", org, appName)
		uploadPath := testenv.TestAssetPath("sample-app.tar")
		uploadRequest, err := uploadRequest(uploadURL, uploadPath)
		Expect(err).ToNot(HaveOccurred())
		resp, err := env.Client().Do(uploadRequest)
		Expect(err).ToNot(HaveOccurred())
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		respObj := &models.UploadResponse{}
		err = json.Unmarshal(bodyBytes, &respObj)
		Expect(err).ToNot(HaveOccurred())

		return respObj
	}

	// returns all the objects currently stored on the S3 storage
	listS3Blobs := func() []string {
		out, err := helpers.Kubectl("get", "secret",
			"-n", "minio-epinio",
			"tenant-creds", "-o", "jsonpath={.data.accesskey}")
		Expect(err).ToNot(HaveOccurred(), out)
		accessKey, err := base64.StdEncoding.DecodeString(string(out))
		Expect(err).ToNot(HaveOccurred(), string(out))

		out, err = helpers.Kubectl("get", "secret",
			"-n", "minio-epinio",
			"tenant-creds", "-o", "jsonpath={.data.secretkey}")
		Expect(err).ToNot(HaveOccurred(), out)
		secretKey, err := base64.StdEncoding.DecodeString(string(out))
		Expect(err).ToNot(HaveOccurred(), string(out))

		rand, err := randstr.Hex16()
		Expect(err).ToNot(HaveOccurred(), out)
		// Setup "mc" to talk to our minio endpoint (the "mc alias" command)
		// and list all objects in the bucket (the "mc --quiet ls" command)
		out, err = helpers.Kubectl("run", "-it",
			"--restart=Never", "miniocli"+rand, "--rm",
			"--image=minio/mc", "--command", "--",
			"/bin/bash", "-c",
			fmt.Sprintf("mc alias set minio http://minio.minio-epinio.svc.cluster.local %s %s 2>&1 > /dev/null && mc --quiet ls minio/epinio", string(accessKey), string(secretKey)))
		Expect(err).ToNot(HaveOccurred(), out)

		return strings.Split(string(out), "\n")
	}

	stageApplication := func(appName, org string, uploadResponse *models.UploadResponse) *models.StageResponse {
		request := models.StageRequest{
			App: models.AppRef{
				Name: appName,
				Org:  org,
			},
			BlobUID:      uploadResponse.BlobUID,
			BuilderImage: "paketobuildpacks/builder:full",
		}
		b, err := json.Marshal(request)
		Expect(err).NotTo(HaveOccurred())
		body := string(b)

		url := serverURL + "/" + v1.Routes.Path("AppStage", org, appName)
		response, err := env.Curl("POST", url, strings.NewReader(body))
		Expect(err).NotTo(HaveOccurred())

		b, err = ioutil.ReadAll(response.Body)
		Expect(err).NotTo(HaveOccurred())

		stage := &models.StageResponse{}
		err = json.Unmarshal(b, stage)
		Expect(err).NotTo(HaveOccurred())

		waitForPipeline(stage.Stage.ID)

		return stage
	}

	BeforeEach(func() {
		org = catalog.NewOrgName()
		env.SetupAndTargetOrg(org)

		// Wait for server to be up and running
		Eventually(func() error {
			_, err := env.Curl("GET", serverURL+"/api/v1/info", strings.NewReader(""))
			return err
		}, "1m").ShouldNot(HaveOccurred())
	})

	Context("Apps", func() {
		Describe("POST /namespaces/:org/applications/:app/import-git", func() {
			It("imports the git repo in the blob store", func() {
				app := catalog.NewAppName()
				gitURL := "https://github.com/epinio/example-wordpress"
				data := url.Values{}
				data.Set("giturl", gitURL)
				data.Set("gitrev", "main")

				url := serverURL + "/" + v1.Routes.Path("AppImportGit", org, app)
				request, err := http.NewRequest("POST", url, strings.NewReader(data.Encode()))
				Expect(err).ToNot(HaveOccurred())
				request.SetBasicAuth(env.EpinioUser, env.EpinioPassword)
				request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
				request.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

				response, err := env.Client().Do(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred(), string(bodyBytes))
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				var importResponse models.ImportGitResponse
				err = json.Unmarshal(bodyBytes, &importResponse)
				Expect(err).ToNot(HaveOccurred())
				Expect(importResponse.BlobUID).ToNot(BeEmpty())
				Expect(importResponse.BlobUID).To(MatchRegexp(".+-.+-.+-.+-.+"))
			})
		})

		Describe("PATCH /namespaces/:org/applications/:app", func() {
			When("instances is valid integer", func() {
				It("updates an application with the desired number of instances", func() {
					app := catalog.NewAppName()
					env.MakeDockerImageApp(app, 1, dockerImageURL)
					defer env.DeleteApp(app)

					Expect(appStatus(org, app)).To(Equal("1/1"))

					status, _ := updateAppInstances(org, app, 3)
					Expect(status).To(Equal(http.StatusOK))

					Eventually(func() string {
						return appStatus(org, app)
					}, "1m").Should(Equal("3/3"))
				})
			})

			When("instances is invalid", func() {
				It("returns BadRequest", func() {
					app := catalog.NewAppName()
					env.MakeDockerImageApp(app, 1, dockerImageURL)
					defer env.DeleteApp(app)
					Expect(appStatus(org, app)).To(Equal("1/1"))

					status, updateResponseBody := updateAppInstances(org, app, -3)
					Expect(status).To(Equal(http.StatusBadRequest))

					var errorResponse v1.ErrorResponse
					err := json.Unmarshal(updateResponseBody, &errorResponse)
					Expect(err).ToNot(HaveOccurred())
					Expect(errorResponse.Errors[0].Status).To(Equal(http.StatusBadRequest))
					Expect(errorResponse.Errors[0].Title).To(Equal("instances param should be integer equal or greater than zero"))
				})
			})

		})

		Describe("GET api/v1/namespaces/:orgs/applications", func() {
			It("lists all applications belonging to the namespace", func() {
				app1 := catalog.NewAppName()
				env.MakeDockerImageApp(app1, 1, dockerImageURL)
				defer env.DeleteApp(app1)
				app2 := catalog.NewAppName()
				env.MakeDockerImageApp(app2, 1, dockerImageURL)
				defer env.DeleteApp(app2)

				response, err := env.Curl("GET", fmt.Sprintf("%s/api/v1/namespaces/%s/applications",
					serverURL, org), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				var apps models.AppList
				err = json.Unmarshal(bodyBytes, &apps)
				Expect(err).ToNot(HaveOccurred())

				appNames := []string{apps[0].Name, apps[1].Name}
				Expect(appNames).To(ContainElements(app1, app2))

				orgNames := []string{apps[0].Organization, apps[1].Organization}
				Expect(orgNames).To(ContainElements(org, org))

				statuses := []string{apps[0].Status, apps[1].Status}
				Expect(statuses).To(ContainElements("1/1", "1/1"))
			})

			It("returns a 404 when the org does not exist", func() {
				response, err := env.Curl("GET", fmt.Sprintf("%s/api/v1/namespaces/idontexist/applications", serverURL), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			})
		})

		Describe("GET api/v1/namespaces/:org/applications/:app", func() {
			It("lists the application data", func() {
				app := catalog.NewAppName()
				env.MakeDockerImageApp(app, 1, dockerImageURL)
				defer env.DeleteApp(app)

				Expect(appStatus(org, app)).To(Equal("1/1"))
			})

			It("returns a 404 when the org does not exist", func() {
				app := catalog.NewAppName()
				env.MakeDockerImageApp(app, 1, dockerImageURL)
				defer env.DeleteApp(app)

				response, err := env.Curl("GET", fmt.Sprintf("%s/api/v1/namespaces/idontexist/applications/%s", serverURL, app), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			})

			It("returns a 404 when the app does not exist", func() {
				response, err := env.Curl("GET", fmt.Sprintf("%s/api/v1/namespaces/%s/applications/bogus", serverURL, org), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			})
		})

		Describe("DELETE api/v1/namespaces/:org/applications/:app", func() {
			It("removes the application, unbinds bound services", func() {
				app1 := catalog.NewAppName()
				env.MakeDockerImageApp(app1, 1, dockerImageURL)
				service := catalog.NewServiceName()
				env.MakeCustomService(service)
				env.BindAppService(app1, service, org)
				defer env.CleanupService(service)

				response, err := env.Curl("DELETE", fmt.Sprintf("%s/api/v1/namespaces/%s/applications/%s", serverURL, org, app1), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())

				var resp map[string][]string
				err = json.Unmarshal(bodyBytes, &resp)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp).To(HaveLen(1))
				Expect(resp).To(HaveKey("unboundservices"))
				Expect(resp["unboundservices"]).To(ContainElement(service))
			})

			It("returns a 404 when the org does not exist", func() {
				app1 := catalog.NewAppName()
				env.MakeDockerImageApp(app1, 1, dockerImageURL)
				defer env.DeleteApp(app1)

				response, err := env.Curl("DELETE", fmt.Sprintf("%s/api/v1/namespaces/idontexist/applications/%s", serverURL, app1), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			})

			It("returns a 404 when the app does not exist", func() {
				response, err := env.Curl("DELETE", fmt.Sprintf("%s/api/v1/namespaces/%s/applications/bogus", serverURL, org), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			})
		})

		Describe("GET api/v1/applications", func() {
			var org1 string
			var org2 string
			var app1 string
			var app2 string

			BeforeEach(func() {
				org1 = catalog.NewOrgName()
				env.SetupAndTargetOrg(org1)

				app1 = catalog.NewAppName()
				env.MakeDockerImageApp(app1, 1, dockerImageURL)

				org2 = catalog.NewOrgName()
				env.SetupAndTargetOrg(org2)

				app2 = catalog.NewAppName()
				env.MakeDockerImageApp(app2, 1, dockerImageURL)
			})
			AfterEach(func() {
				env.TargetOrg(org2)
				env.DeleteApp(app2)

				env.TargetOrg(org1)
				env.DeleteApp(app1)
			})
			It("lists all applications belonging to all namespaces", func() {
				response, err := env.Curl("GET", fmt.Sprintf("%s/api/v1/applications",
					serverURL), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				var apps models.AppList
				err = json.Unmarshal(bodyBytes, &apps)
				Expect(err).ToNot(HaveOccurred())

				// `apps` contains all apps. Not just the two we are looking for, from
				// the setup of this test. Everything which still exists from other
				// tests executing concurrently, or not cleaned by previous tests, or
				// the setup, or ... So, we cannot be sure that the two apps are in the
				// two first elements of the slice.

				var appRefs [][]string
				for _, a := range apps {
					appRefs = append(appRefs, []string{a.Name, a.Organization})
				}
				Expect(appRefs).To(ContainElements(
					[]string{app1, org1},
					[]string{app2, org2}))
			})
		})
	})

	Context("Uploading", func() {

		var (
			url     string
			path    string
			request *http.Request
		)

		JustBeforeEach(func() {
			url = serverURL + "/" + v1.Routes.Path("AppUpload", org, "testapp")
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
			org = catalog.NewOrgName()
			env.SetupAndTargetOrg(org)
			appName = catalog.NewAppName()

			By("creating application resource first")
			_, err := createApplication(appName, org)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			env.DeleteApp(appName)
		})

		Context("with staging", func() {
			When("staging the same app with a new blob", func() {
				It("cleans up old S3 objects", func() {
					By("uploading the code")
					uploadResponse := uploadApplication(appName)
					oldBlob := uploadResponse.BlobUID
					By("staging the application")
					_ = stageApplication(appName, org, uploadResponse)
					Eventually(listS3Blobs, "1m").Should(ContainElement(ContainSubstring(oldBlob)))

					By("uploading the code again")
					uploadResponse = uploadApplication(appName)
					newBlob := uploadResponse.BlobUID
					By("staging the application again")
					_ = stageApplication(appName, org, uploadResponse)

					Eventually(listS3Blobs, "1m").Should(ContainElement(ContainSubstring(newBlob)))
					Eventually(listS3Blobs, "1m").ShouldNot(ContainElement(ContainSubstring(oldBlob)))
				})
			})

			When("deploying a new app", func() {
				It("returns a success", func() {
					By("uploading the code")
					uploadResponse := uploadApplication(appName)

					By("staging the application")
					stageResponse := stageApplication(appName, org, uploadResponse)

					By("deploying the staged resource")
					request = models.DeployRequest{
						App: models.AppRef{
							Name: appName,
							Org:  org,
						},
						Instances: &one,
						Stage: models.StageRef{
							ID: stageResponse.Stage.ID,
						},
						ImageURL: stageResponse.ImageURL,
					}

					bodyBytes, err := json.Marshal(request)
					Expect(err).ToNot(HaveOccurred())
					body = string(bodyBytes)

					url = serverURL + "/" + v1.Routes.Path("AppDeploy", org, appName)

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
					Expect(deploy.Route).To(MatchRegexp(appName + `.*\.omg\.howdoi\.website`))

					By("waiting for the deployment to complete")

					url = serverURL + "/" + v1.Routes.Path("AppRunning", org, appName)

					response, err = env.Curl("GET", url, strings.NewReader(body))
					Expect(err).ToNot(HaveOccurred())
					Expect(response).ToNot(BeNil())
					defer response.Body.Close()

					By("confirming at highlevel")
					// Highlevel check and confirmation
					Eventually(func() string {
						return appStatus(org, appName)
					}, "5m").Should(Equal("1/1"))
				})
			})
		})

		Context("with non-staging using custom docker image", func() {
			BeforeEach(func() {
				request = models.DeployRequest{
					App: models.AppRef{
						Name: appName,
						Org:  org,
					},
					Instances: &one,
					ImageURL:  "splatform/sample-app",
				}

				url = serverURL + "/" + v1.Routes.Path("AppDeploy", org, appName)
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
					Expect(deploy.Route).To(MatchRegexp(appName + `.*\.omg\.howdoi\.website`))

					Eventually(func() string {
						return appStatus(org, appName)
					}, "5m").Should(Equal("1/1"))

					// Check if autoserviceaccounttoken is true
					labels := fmt.Sprintf("app.kubernetes.io/name=%s", appName)
					out, err := helpers.Kubectl("get", "pod",
						"--namespace", org,
						"-l", labels,
						"-o", "jsonpath={.items[*].spec.automountServiceAccountToken}")
					Expect(err).NotTo(HaveOccurred())
					Expect(out).To(ContainSubstring("true"))
				})
			})

			When("deloying with more instances", func() {
				BeforeEach(func() {
					request.Instances = &two
					bodyBytes, err := json.Marshal(request)
					Expect(err).ToNot(HaveOccurred())
					body = string(bodyBytes)
				})

				It("creates an app with the specified number of instances", func() {
					response, err := env.Curl("POST", url, strings.NewReader(body))
					Expect(err).ToNot(HaveOccurred())
					Expect(response).ToNot(BeNil())
					defer response.Body.Close()

					Eventually(func() string {
						return appStatus(org, appName)
					}, "5m").Should(Equal("2/2"))
				})
			})

			When("deploying with invalid instances", func() {
				When("instances is a negative integer", func() {
					BeforeEach(func() {
						n := int32(-3)
						request.Instances = &n
						bodyBytes, err := json.Marshal(request)
						Expect(err).ToNot(HaveOccurred())
						body = string(bodyBytes)
					})

					It("returns BadRequest", func() {
						resp, err := env.Curl("POST", url, strings.NewReader(body))
						Expect(err).ToNot(HaveOccurred())
						Expect(resp).ToNot(BeNil())
						defer resp.Body.Close()

						bodyBytes, err := ioutil.ReadAll(resp.Body)
						Expect(err).ToNot(HaveOccurred())
						Expect(resp.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))

						r := &v1.ErrorResponse{}
						err = json.Unmarshal(bodyBytes, &r)
						Expect(err).ToNot(HaveOccurred())

						responseErr := r.Errors[0]
						Expect(responseErr.Status).To(Equal(400))
						Expect(responseErr.Title).To(Equal("instances param should be integer equal or greater than zero"))
					})
				})

				When("instances is not a number", func() {
					BeforeEach(func() {
						n := int32(314)
						request.Instances = &n // Hack: see below too
						bodyBytes, err := json.Marshal(request)
						Expect(err).ToNot(HaveOccurred())
						body = string(bodyBytes)
					})

					It("returns BadRequest", func() {
						// Hack to make the Instances value non-number
						body = strings.Replace(body, "314", "thisisnotanumber", 1)

						resp, err := env.Curl("POST", url, strings.NewReader(body))
						Expect(err).ToNot(HaveOccurred())
						Expect(resp).ToNot(BeNil())
						defer resp.Body.Close()

						bodyBytes, err := ioutil.ReadAll(resp.Body)
						Expect(err).ToNot(HaveOccurred())
						Expect(resp.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))

						r := &v1.ErrorResponse{}
						err = json.Unmarshal(bodyBytes, &r)
						Expect(err).ToNot(HaveOccurred())

						responseErr := r.Errors[0]
						Expect(responseErr.Status).To(Equal(400))
						Expect(responseErr.Title).To(ContainSubstring("Failed to unmarshal deploy request"))
					})
				})
			})
		})
	})

	Context("Logs", func() {
		Describe("GET api/v1/namespaces/:orgs/applications/:app/logs", func() {
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

			readLogs := func(org, app string) string {
				var urlArgs = []string{}
				urlArgs = append(urlArgs, fmt.Sprintf("follow=%t", false))
				wsURL := fmt.Sprintf("%s/%s?%s", websocketURL, v1.Routes.Path("AppLogs", org, app), strings.Join(urlArgs, "&"))
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
				logs := readLogs(org, app)

				By("checking if the logs are right")
				podNames := env.GetPodNames(app, org)
				for _, podName := range podNames {
					Expect(logs).To(ContainSubstring(podName))
				}
			})

			It("should follow logs", func() {
				existingLogs := readLogs(org, app)
				logLength := len(strings.Split(existingLogs, "\n"))

				var urlArgs = []string{}
				urlArgs = append(urlArgs, fmt.Sprintf("follow=%t", true))
				wsURL := fmt.Sprintf("%s/%s?%s", websocketURL, v1.Routes.Path("AppLogs", org, app), strings.Join(urlArgs, "&"))
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
			org = catalog.NewOrgName()
			env.SetupAndTargetOrg(org)
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
				response, err := createApplication(appName, org)
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()

				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
			})
		})
	})
})
