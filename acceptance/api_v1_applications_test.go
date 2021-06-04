package acceptance_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("Apps API Application Endpoints", func() {
	var (
		org string
		one int32 = 1
		two int32 = 2
	)

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
		request.SetBasicAuth(epinioUser, epinioPassword)
		if err != nil {
			return nil, errors.Wrap(err, "failed to build request")
		}
		request.Header.Add("Content-Type", writer.FormDataContentType())

		return request, nil
	}

	appStatus := func(org, app string) string {
		response, err := Curl("GET",
			fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s", serverURL, org, app),
			strings.NewReader(""))

		ExpectWithOffset(1, err).ToNot(HaveOccurred())
		ExpectWithOffset(1, response).ToNot(BeNil())
		defer response.Body.Close()
		ExpectWithOffset(1, response.StatusCode).To(Equal(http.StatusOK))
		bodyBytes, err := ioutil.ReadAll(response.Body)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())

		var responseApp application.Application
		err = json.Unmarshal(bodyBytes, &responseApp)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
		ExpectWithOffset(1, responseApp.Name).To(Equal(app))
		ExpectWithOffset(1, responseApp.Organization).To(Equal(org))

		return responseApp.Status
	}

	updateAppInstances := func(org string, app string, instances int32) (int, []byte) {
		data, err := json.Marshal(models.UpdateAppRequest{Instances: instances})
		ExpectWithOffset(1, err).ToNot(HaveOccurred())

		response, err := Curl("PATCH",
			fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s", serverURL, org, app),
			strings.NewReader(string(data)))
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
		ExpectWithOffset(1, response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())

		return response.StatusCode, bodyBytes
	}

	BeforeEach(func() {
		org = newOrgName()
		setupAndTargetOrg(org)

		// Wait for server to be up and running
		Eventually(func() error {
			_, err := Curl("GET", serverURL+"/api/v1/info", strings.NewReader(""))
			return err
		}, "1m").ShouldNot(HaveOccurred())
	})

	Context("Apps", func() {
		Describe("PATCH /orgs/:org/applications/:app", func() {
			When("instances is valid integer", func() {
				It("updates an application with the desired number of instances", func() {
					app := newAppName()
					makeApp(app, 1, true)
					defer deleteApp(app)

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
					app := newAppName()
					makeApp(app, 1, true)
					defer deleteApp(app)
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

		Describe("GET api/v1/orgs/:orgs/applications", func() {
			It("lists all applications belonging to the org", func() {
				app1 := newAppName()
				makeApp(app1, 1, true)
				defer deleteApp(app1)
				app2 := newAppName()
				makeApp(app2, 1, true)
				defer deleteApp(app2)

				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/%s/applications",
					serverURL, org), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				var apps application.ApplicationList
				err = json.Unmarshal(bodyBytes, &apps)
				Expect(err).ToNot(HaveOccurred())
				Expect(apps[0].Name).To(Equal(app1))
				Expect(apps[0].Organization).To(Equal(org))
				Expect(apps[0].Status).To(Equal("1/1"))
				Expect(apps[1].Name).To(Equal(app2))
				Expect(apps[1].Organization).To(Equal(org))
				Expect(apps[1].Status).To(Equal("1/1"))
			})

			It("returns a 404 when the org does not exist", func() {
				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/idontexist/applications", serverURL), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			})
		})

		Describe("GET api/v1/orgs/:org/applications/:app", func() {
			It("lists the application data", func() {
				app := newAppName()
				makeApp(app, 1, true)
				defer deleteApp(app)

				Expect(appStatus(org, app)).To(Equal("1/1"))
			})

			It("returns a 404 when the org does not exist", func() {
				app := newAppName()
				makeApp(app, 1, true)
				defer deleteApp(app)

				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/idontexist/applications/%s", serverURL, app), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			})

			It("returns a 404 when the app does not exist", func() {
				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/%s/applications/bogus", serverURL, org), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			})
		})

		Describe("DELETE api/v1/orgs/:org/applications/:app", func() {
			It("removes the application, unbinds bound services", func() {
				app1 := newAppName()
				makeApp(app1, 1, true)
				service := newServiceName()
				makeCustomService(service)
				bindAppService(app1, service, org)
				defer cleanupService(service)

				response, err := Curl("DELETE", fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s", serverURL, org, app1), strings.NewReader(""))
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
				Expect(resp).To(HaveKey("UnboundServices"))
				Expect(resp["UnboundServices"]).To(ContainElement(service))
			})

			It("returns a 404 when the org does not exist", func() {
				app1 := newAppName()
				makeApp(app1, 1, true)
				defer deleteApp(app1)

				response, err := Curl("DELETE", fmt.Sprintf("%s/api/v1/orgs/idontexist/applications/%s", serverURL, app1), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			})

			It("returns a 404 when the app does not exist", func() {
				response, err := Curl("DELETE", fmt.Sprintf("%s/api/v1/orgs/%s/applications/bogus", serverURL, org), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
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

		When("uploading a broken tarball", func() {
			BeforeEach(func() {
				path = "../fixtures/untar.tgz"
			})

			It("returns an error response", func() {
				resp, err := Client().Do(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				defer resp.Body.Close()

				bodyBytes, err := ioutil.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError), string(bodyBytes))

				r := &v1.ErrorResponse{}
				err = json.Unmarshal(bodyBytes, &r)
				Expect(err).ToNot(HaveOccurred())

				Expect(r.Errors).To(HaveLen(1))
				Expect(r.Errors[0].Details).To(ContainSubstring("failed to unpack"))
			})
		})

		When("uploading a new dir", func() {
			BeforeEach(func() {
				path = "../fixtures/sample-app.tar"
			})

			It("returns the app response", func() {
				resp, err := Client().Do(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				defer resp.Body.Close()

				bodyBytes, err := ioutil.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				r := &models.UploadResponse{}
				err = json.Unmarshal(bodyBytes, &r)
				Expect(err).ToNot(HaveOccurred())

				Expect(r.Git.URL).ToNot(BeEmpty())
				Expect(r.Git.Revision).ToNot(BeEmpty())
			})
		})

	})

	Context("Staging", func() {
		var (
			url     string
			body    string
			appName string
			request models.StageRequest
		)

		BeforeEach(func() {
			org = newOrgName()
			setupAndTargetOrg(org)
			appName = newAppName()

			// First upload to allow staging to succeed
			uploadURL := serverURL + "/" + v1.Routes.Path("AppUpload", org, appName)
			uploadPath := "../fixtures/sample-app.tar"
			uploadRequest, err := uploadRequest(uploadURL, uploadPath)
			Expect(err).ToNot(HaveOccurred())
			resp, err := Client().Do(uploadRequest)
			Expect(err).ToNot(HaveOccurred())
			bodyBytes, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			respObj := &models.UploadResponse{}
			err = json.Unmarshal(bodyBytes, &respObj)
			Expect(err).ToNot(HaveOccurred())

			request = models.StageRequest{
				App: models.AppRef{
					Name: appName,
					Org:  org,
				},
				Instances: &one,
				Git: &models.GitRef{
					Revision: respObj.Git.Revision,
					URL:      respObj.Git.URL,
				},
				Route: appName + ".omg.howdoi.website",
			}

			url = serverURL + "/" + v1.Routes.Path("AppStage", org, appName)
		})

		JustBeforeEach(func() {
			bodyBytes, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())
			body = string(bodyBytes)
		})

		When("staging a new app", func() {
			It("returns a success", func() {
				defer func() { // Cleanup
					Eventually(func() error {
						_, err := Epinio("app delete "+appName, "")
						return err
					}, "5m").ShouldNot(HaveOccurred())
				}()

				response, err := Curl("POST", url, strings.NewReader(body))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()

				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
			})
		})

		When("staging with more instances", func() {
			BeforeEach(func() {
				request.Instances = &two
			})

			It("creates an app with the specified number of instances", func() {
				defer deleteApp(appName)

				response, err := Curl("POST", url, strings.NewReader(body))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()

				Eventually(func() int {
					response, err := Curl("GET",
						fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s", serverURL, org, appName),
						strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					Expect(response).ToNot(BeNil())
					defer response.Body.Close()
					return response.StatusCode
				}, "5m").Should(Equal(200))

				Eventually(func() string {
					return appStatus(org, appName)
				}, "2m").Should(Equal("2/2"))
			})
		})

		When("staging with invalid instances", func() {
			When("instances is not a integer", func() {
				BeforeEach(func() {
					n := int32(314)
					request.Instances = &n // Hack: see below too
				})

				It("returns BadRequest", func() {
					// Hack to make the Instances value non-integer
					body = strings.Replace(body, "314", "3.14", 1)

					resp, err := Curl("POST", url, strings.NewReader(body))
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
					Expect(responseErr.Title).To(Equal("Failed to construct an Application from the request"))
					Expect(responseErr.Details).To(MatchRegexp(
						"cannot unmarshal number 3.14 into Go struct field StageRequest.instances of type int",
					))
				})
			})

			When("instances is a negative integer", func() {
				BeforeEach(func() {
					n := int32(-3)
					request.Instances = &n
				})

				It("returns BadRequest", func() {
					resp, err := Curl("POST", url, strings.NewReader(body))
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
				})

				It("returns BadRequest", func() {
					// Hack to make the Instances value non-number
					body = strings.Replace(body, "314", "thisisnotanumber", 1)

					resp, err := Curl("POST", url, strings.NewReader(body))
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
					Expect(responseErr.Title).To(Equal("Failed to construct an Application from the request"))
				})
			})
		})
	})

	Context("Logs", func() {
		Describe("GET api/v1/orgs/:orgs/applications/:app/logs", func() {
			logLength := 0
			var (
				route string
				app   string
			)

			BeforeEach(func() {
				app = newAppName()
				out := makeApp(app, 1, true)
				routeRegexp := regexp.MustCompile(`https:\/\/.*omg.howdoi.website`)
				route = string(routeRegexp.Find([]byte(out)))
			})

			AfterEach(func() {
				deleteApp(app)
			})

			readLogs := func(org, app string) string {
				var urlArgs = []string{}
				urlArgs = append(urlArgs, fmt.Sprintf("follow=%t", false))
				wsURL := fmt.Sprintf("%s/%s?%s", websocketURL, v1.Routes.Path("AppLogs", org, app), strings.Join(urlArgs, "&"))
				wsConn := makeWebSocketConnection(wsURL)

				By("read the logs")
				var logs string
				Eventually(func() bool {
					_, message, err := wsConn.ReadMessage()
					logLength++
					logs = fmt.Sprintf("%s %s", logs, string(message))
					return websocket.IsCloseError(err, websocket.CloseNormalClosure)
				}, 30*time.Second, 1*time.Second).Should(BeTrue())

				err := wsConn.Close()
				Expect(err).ToNot(HaveOccurred())

				return logs
			}

			It("should send the logs", func() {
				logs := readLogs(org, app)

				By("checking if the logs are right")
				podNames := getPodNames(app, org)
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
				wsConn := makeWebSocketConnection(wsURL)

				By("get to the end of logs")
				for i := 0; i < logLength-1; i++ {
					_, message, err := wsConn.ReadMessage()
					Expect(err).NotTo(HaveOccurred())
					Expect(message).NotTo(BeNil())
				}

				By("adding more logs")
				Eventually(func() int {
					resp, err := Curl("GET", route, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					return resp.StatusCode
				}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

				By("checking the latest log message")
				Eventually(func() string {
					_, message, err := wsConn.ReadMessage()
					Expect(err).NotTo(HaveOccurred())
					Expect(message).NotTo(BeNil())
					return string(message)
				}).Should(ContainSubstring("GET / HTTP/1.1"))

				err := wsConn.Close()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
