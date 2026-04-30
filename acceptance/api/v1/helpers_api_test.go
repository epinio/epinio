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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	v1 "github.com/epinio/epinio/internal/api/v1"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func appShow(namespace, app string) models.App {
	GinkgoHelper()

	endpoint := makeEndpoint(v1.Routes.Path("AppShow", namespace, app))
	bodyBytes, statusCode := curl(http.MethodGet, endpoint, nil)
	Expect(statusCode).To(Equal(http.StatusOK))

	responseApp := fromJSON[models.App](bodyBytes)
	Expect(responseApp.Meta.Name).To(Equal(app))
	Expect(responseApp.Meta.Namespace).To(Equal(namespace))

	return responseApp
}

func appCreate(namespace string, body io.Reader) ([]byte, int) {
	GinkgoHelper()

	endpoint := makeEndpoint(v1.Routes.Path("AppCreate", namespace))
	return curl(http.MethodPost, endpoint, body)
}

func appUpdate(namespace, app string, body io.Reader) ([]byte, int) {
	GinkgoHelper()

	endpoint := makeEndpoint(v1.Routes.Path("AppUpdate", namespace, app))
	return curl(http.MethodPatch, endpoint, body)
}

func appValidateCV(namespace, app string) ([]byte, int) {
	GinkgoHelper()

	endpoint := makeEndpoint(v1.Routes.Path("AppValidateCV", namespace, app))
	return curl(http.MethodGet, endpoint, nil)
}

func appDeploy(namespace, app string, body io.Reader) ([]byte, int) {
	GinkgoHelper()

	endpoint := makeEndpoint(v1.Routes.Path("AppDeploy", namespace, app))
	return curl(http.MethodPost, endpoint, body)
}

func appImportGit(namespace, app, gitURL, revision string) ([]byte, int) {
	GinkgoHelper()

	data := url.Values{}
	data.Set("giturl", gitURL)
	data.Set("gitrev", revision)

	endpoint := makeEndpoint(v1.Routes.Path("AppImportGit", namespace, app))
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	Expect(err).ToNot(HaveOccurred())

	request.SetBasicAuth(env.EpinioUser, env.EpinioPassword)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	response, err := env.Client().Do(request)
	Expect(err).ToNot(HaveOccurred())
	Expect(response).ToNot(BeNil())
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	Expect(err).ToNot(HaveOccurred())

	return bodyBytes, response.StatusCode
}

func gitproxy(body io.Reader) ([]byte, int) {
	GinkgoHelper()

	endpoint := makeEndpoint(v1.Routes.Path("GitProxy"))
	return curl(http.MethodPost, endpoint, body)
}

func me() ([]byte, int) {
	GinkgoHelper()

	endpoint := makeEndpoint("me")
	return curl(http.MethodGet, endpoint, nil)
}

func curl(method, endpoint string, body io.Reader) ([]byte, int) {
	GinkgoHelper()

	response, err := env.Curl(method, endpoint, body)
	Expect(err).ToNot(HaveOccurred())
	Expect(response).ToNot(BeNil())
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	Expect(err).ToNot(HaveOccurred())

	return bodyBytes, response.StatusCode
}

func toJSON(request any) io.Reader {
	GinkgoHelper()

	b, err := json.Marshal(request)
	Expect(err).ToNot(HaveOccurred())
	return bytes.NewReader(b)
}

func fromJSON[T any](bodyBytes []byte) T {
	GinkgoHelper()

	t := new(T)
	err := json.Unmarshal(bodyBytes, t)
	Expect(err).ToNot(HaveOccurred())

	return *t
}

func makeEndpoint(path string) string {
	return fmt.Sprintf("%s%s/%s", serverURL, v1.Root, path)
}

func ExpectResponseToBeOK(bodyBytes []byte, statusCode int) {
	GinkgoHelper()

	Expect(statusCode).To(Equal(http.StatusOK), string(bodyBytes))

	response := fromJSON[models.Response](bodyBytes)
	Expect(response).To(Equal(models.ResponseOK))
}

func ExpectBadRequestError(bodyBytes []byte, statusCode int, expectedErrorMsg string) {
	GinkgoHelper()

	Expect(statusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))

	errorResponse := fromJSON[apierrors.ErrorResponse](bodyBytes)
	Expect(errorResponse.Errors[0].Title).To(Equal(expectedErrorMsg))
}
