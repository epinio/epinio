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

package v1_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	v1 "github.com/epinio/epinio/internal/api/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// makeSourceTar builds an in-memory plain tar holding the given files.
func makeSourceTar(files map[string]string) *bytes.Buffer {
	buffer := &bytes.Buffer{}
	tarWriter := tar.NewWriter(buffer)

	for name, content := range files {
		header := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		writeHeaderError := tarWriter.WriteHeader(header)
		Expect(writeHeaderError).ToNot(HaveOccurred())
		_, writeError := tarWriter.Write([]byte(content))
		Expect(writeError).ToNot(HaveOccurred())
	}

	closeError := tarWriter.Close()
	Expect(closeError).ToNot(HaveOccurred())

	return buffer
}

// makeMultipartRequest builds an authenticated multipart request carrying the
// tar as the "file" part plus the given form fields.
func makeMultipartRequest(
	method, url string,
	tarData *bytes.Buffer,
	fields map[string]string,
) *http.Request {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if tarData != nil {
		part, createFormError := writer.CreateFormFile("file", "source.tar")
		Expect(createFormError).ToNot(HaveOccurred())
		_, copyError := io.Copy(part, tarData)
		Expect(copyError).ToNot(HaveOccurred())
	}

	for key, value := range fields {
		writeFieldError := writer.WriteField(key, value)
		Expect(writeFieldError).ToNot(HaveOccurred())
	}

	closeError := writer.Close()
	Expect(closeError).ToNot(HaveOccurred())

	request, newRequestError := http.NewRequest(method, url, body)
	Expect(newRequestError).ToNot(HaveOccurred())
	request.SetBasicAuth(env.EpinioUser, env.EpinioPassword)
	request.Header.Add("Content-Type", writer.FormDataContentType())

	return request
}

var _ = Describe("AppSourcePatch and AppSync Endpoints",
	LApplication, Ordered, func() {
		var (
			namespace string
			appName   string
			route     string
		)

		curlApp := func() string {
			response, curlError := env.Curl(
				"GET",
				route,
				strings.NewReader(""),
			)

			if curlError != nil {
				return ""
			}

			defer func() {
				_ = response.Body.Close()
			}()

			content, readError := io.ReadAll(response.Body)
			if readError != nil {
				return ""
			}

			return string(content)
		}

		BeforeAll(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			appName = catalog.NewAppName()
			pushOutput := env.MakeApp(appName, 1, true)

			route = testenv.AppRouteWithPort(
				testenv.AppRouteFromOutput(pushOutput),
			)
			Expect(route).ToNot(BeEmpty(), pushOutput)
		})

		AfterAll(func() {
			env.DeleteApp(appName)
			env.DeleteNamespace(namespace)
		})

		Describe("AppSync input validation", func() {

			It("rejects an invalid mode with 400", func() {
				url := fmt.Sprintf(
					"%s%s/%s",
					serverURL,
					v1.Root,
					v1.Routes.Path("AppSync", namespace, appName),
				)

				request := makeMultipartRequest(
					"POST",
					url,
					makeSourceTar(map[string]string{"x": "y"}),
					map[string]string{"mode": "bogus"},
				)

				response, requestError := env.Client().Do(request)
				Expect(requestError).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(
					Equal(http.StatusBadRequest),
				)
			})

			It("rejects a request without a file with 400", func() {
				url := fmt.Sprintf(
					"%s%s/%s",
					serverURL,
					v1.Root,
					v1.Routes.Path("AppSync", namespace, appName),
				)

				request := makeMultipartRequest(
					"POST",
					url,
					nil,
					map[string]string{"mode": "files"},
				)

				response, requestError := env.Client().Do(request)
				Expect(requestError).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(
					Equal(http.StatusBadRequest),
				)
			})

			It("fails with 503 when the app has no running pod", func() {
				bareApp := catalog.NewAppName()
				createOutput, createError := env.Epinio(
					"",
					"app",
					"create",
					bareApp,
				)
				Expect(createError).ToNot(HaveOccurred(), createOutput)

				url := fmt.Sprintf(
					"%s%s/%s",
					serverURL,
					v1.Root,
					v1.Routes.Path("AppSync", namespace, bareApp),
				)

				request := makeMultipartRequest(
					"POST",
					url,
					makeSourceTar(map[string]string{"x": "y"}),
					map[string]string{"mode": "files"},
				)

				response, requestError := env.Client().Do(request)
				Expect(requestError).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(
					Equal(http.StatusServiceUnavailable),
				)

				env.DeleteApp(bareApp)
			})
		})

		Describe("source patch then file sync", func() {

			It("patches the app source and installs the supervisor", func() {
				sourceTar := makeSourceTar(map[string]string{
					"index.php": "<?php echo \"Hello Patched World\"; ?>",
				})

				url := fmt.Sprintf(
					"%s%s/%s",
					serverURL,
					v1.Root,
					v1.Routes.Path("AppSourcePatch", namespace, appName),
				)
				request := makeMultipartRequest("PATCH", url, sourceTar, nil)

				response, requestError := env.Client().Do(request)
				Expect(requestError).ToNot(HaveOccurred())

				responseBody, readError := io.ReadAll(response.Body)
				Expect(readError).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(
					Equal(http.StatusOK), string(responseBody),
				)

				By("waiting for the patched content to be served")
				Eventually(curlApp, "5m", "5s").Should(
					ContainSubstring("Hello Patched World"),
				)

				By("checking the supervisor wrapper is installed")
				Eventually(func() string {
					out, _ := proc.Kubectl(
						"get", "deployment",
						"--namespace", namespace,
						"-l", "app.kubernetes.io/name="+appName,
						"-o",
						"jsonpath={.items[0].spec.template.spec"+
							".containers[0].command}",
					)
					return out
				}, "2m", "5s").Should(ContainSubstring("/epinio-sync"))
			})

			It("syncs changed files into the running pod", func() {
				syncTar := makeSourceTar(map[string]string{
					"index.php": "<?php echo \"Hello Synced World\"; ?>",
				})

				url := fmt.Sprintf(
					"%s%s/%s",
					serverURL,
					v1.Root,
					v1.Routes.Path("AppSync", namespace, appName),
				)
				request := makeMultipartRequest(
					"POST", url, syncTar,
					map[string]string{"mode": "files"},
				)

				response, requestError := env.Client().Do(request)
				Expect(requestError).ToNot(HaveOccurred())

				responseBody, readError := io.ReadAll(response.Body)
				Expect(readError).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(
					Equal(http.StatusOK), string(responseBody),
				)

				By("waiting for the synced content to be served")
				Eventually(curlApp, "2m", "2s").Should(
					ContainSubstring("Hello Synced World"),
				)
			})
		})
	})
