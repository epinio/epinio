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
	"strings"

	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/application"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("Apps API Application Endpoints", func() {

	var org string

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
		Describe("POST /orgs/:org/applications/:app", func() {
			It("deploys an application with the desired number of instances", func() {
				Skip("TODO")
			})
		})

		Describe("PATCH /orgs/:org/applications/:app", func() {
			It("updates an application with the desired number of instances", func() {
				Skip("TODO")
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
				app1 := newAppName()
				makeApp(app1, 1, true)
				defer deleteApp(app1)

				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/%s/applications/%s", serverURL, org, app1), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())

				var app application.Application
				err = json.Unmarshal(bodyBytes, &app)
				Expect(err).ToNot(HaveOccurred())
				Expect(app.Name).To(Equal(app1))
				Expect(app.Organization).To(Equal(org))
				Expect(app.Status).To(Equal("1/1"))
			})

			It("returns a 404 when the org does not exist", func() {
				app1 := newAppName()
				makeApp(app1, 1, true)
				defer deleteApp(app1)

				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/idontexist/applications/%s", serverURL, app1), strings.NewReader(""))
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

		uploadRequest := func(url string, path string) (*http.Request, error) {
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
				resp, err := (&http.Client{}).Do(request)
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
				resp, err := (&http.Client{}).Do(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp).ToNot(BeNil())
				defer resp.Body.Close()

				bodyBytes, err := ioutil.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				r := &v1.AppResponse{}
				err = json.Unmarshal(bodyBytes, &r)
				Expect(err).ToNot(HaveOccurred())

				Expect(r.Message).To(ContainSubstring("ok"))
				Expect(r.App.Route).To(MatchRegexp(`testapp\..*\.omg\.howdoi\.website`))
				Expect(r.App.Name).To(Equal("testapp"))
				Expect(r.App.Org).To(Equal(org))
				Expect(r.App.Repo.URL).ToNot(BeEmpty())
				Expect(r.App.Repo.Revision).ToNot(BeEmpty())
			})
		})
	})

	Context("Staging", func() {
		var (
			url  string
			body string
		)

		BeforeEach(func() {
			url = serverURL + "/" + v1.Routes.Path("AppStage", org, "testapp")
			body = fmt.Sprintf(`{"Name":"testapp","Org":"%s","Repo":{"Revision":"7730c8f3e6490c334397b3125da5173061d656ff","URL":"http://gitea.172.27.0.2.omg.howdoi.website"},"Route":"apps-786195048.172.27.0.2.omg.howdoi.website","ImageID":"9827b03f"}`, org)

		})

		When("staging a new app", func() {
			// but the pipelinerun will fail
			It("returns a success", func() {
				response, err := Curl("POST", url, strings.NewReader(body))
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
