// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client HTTP", func() {

	var epinioClient *client.Client
	var responseBody string
	var statusHeader int
	var requestInterceptor func(r *http.Request)

	JustBeforeEach(func() {
		requestInterceptor = func(r *http.Request) {}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()

			w.WriteHeader(statusHeader)
			fmt.Fprint(w, responseBody)

			requestInterceptor(r)
		}))

		epinioClient = client.New(context.Background(), &settings.Settings{
			API:      srv.URL,
			Location: "fake",
		})
	})

	Describe("executing a request", func() {

		BeforeEach(func() {
			statusHeader = http.StatusOK
		})

		When("custom headers are set", func() {
			It("gets the additional headers", func() {
				responseBody = `{"status":"ok"}`

				epinioClient.SetHeader("x-custom-header-1", "custom header 1")
				epinioClient.SetHeader("x-custom-header-2", "custom header 2")

				// check that Header foo is set
				requestInterceptor = func(r *http.Request) {
					customHeader1 := r.Header.Get("x-custom-header-1")
					Expect(customHeader1).NotTo(BeEmpty())
					Expect(customHeader1).To(Equal("custom header 1"))

					customHeader2 := r.Header.Get("x-custom-header-2")
					Expect(customHeader2).NotTo(BeEmpty())
					Expect(customHeader2).To(Equal("custom header 2"))
				}

				_, err := client.Do(epinioClient, "any", http.MethodGet, nil, &models.Response{})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("no custom headers are set", func() {
			It("gets no additional headers", func() {
				responseBody = `{"status":"ok"}`

				// check that Header foo is set
				requestInterceptor = func(r *http.Request) {
					Expect(r.Header).To(HaveLen(2))

					standardHeader1 := r.Header.Get("Accept-Encoding")
					Expect(standardHeader1).NotTo(BeEmpty())

					standardHeader2 := r.Header.Get("User-Agent")
					Expect(standardHeader2).NotTo(BeEmpty())
				}

				_, err := client.Do(epinioClient, "any", http.MethodGet, nil, &models.Response{})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("executing a failing request", func() {

		BeforeEach(func() {
			statusHeader = http.StatusInternalServerError
		})

		When("server returns an empty response", func() {
			It("fails", func() {
				responseBody = ``

				_, err := client.Do(epinioClient, "any", http.MethodGet, nil, &models.Response{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("empty"))
			})
		})

		When("server returns an error response", func() {
			It("contains the errors", func() {
				responseBody = `{
						"errors": [
							{
								"status": 500,
								"title": "Error title",
								"details": "something bad happened"
							}
						]
					}`

				_, err := client.Do(epinioClient, "any", http.MethodGet, nil, &models.Response{})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("NewFileUploadWithFieldsRequestHandler", func() {

		When("a file and fields are provided", func() {
			It("builds a multipart request with the file and all fields", func() {
				uploadFile, createFileError := os.CreateTemp("", "upload-*.tar")
				Expect(createFileError).ToNot(HaveOccurred())
				defer os.Remove(uploadFile.Name())

				_, writeError := uploadFile.WriteString("tar-content")
				Expect(writeError).ToNot(HaveOccurred())

				_, seekError := uploadFile.Seek(0, io.SeekStart)
				Expect(seekError).ToNot(HaveOccurred())

				handler := client.NewFileUploadWithFieldsRequestHandler(
					uploadFile,
					map[string]string{
						"mode":        "binary",
						"binary_name": "my-app",
					},
				)

				request, handlerError := handler(http.MethodPost, "http://localhost/sync")
				Expect(handlerError).ToNot(HaveOccurred())
				Expect(request.Header.Get("Content-Type")).To(
					ContainSubstring("multipart/form-data"),
				)

				parseError := request.ParseMultipartForm(1 << 20)
				Expect(parseError).ToNot(HaveOccurred())
				Expect(request.FormValue("mode")).To(Equal("binary"))
				Expect(request.FormValue("binary_name")).To(Equal("my-app"))

				formFile, _, formFileError := request.FormFile("file")
				Expect(formFileError).ToNot(HaveOccurred())
				content, readError := io.ReadAll(formFile)
				Expect(readError).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("tar-content"))
			})
		})

		When("no file is provided", func() {
			It("fails", func() {
				handler := client.NewFileUploadWithFieldsRequestHandler(nil, nil)
				_, handlerError := handler(http.MethodPost, "http://localhost/sync")
				Expect(handlerError).To(HaveOccurred())
			})
		})
	})
})
