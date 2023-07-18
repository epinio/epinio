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

	v1 "github.com/epinio/epinio/internal/api/v1"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func appShow(namespace, app string) models.App {
	GinkgoHelper()

	endpoint := makeEndpoint(v1.Routes.Path("AppShow", namespace, app))
	response, err := env.Curl(http.MethodGet, endpoint, nil)

	Expect(err).ToNot(HaveOccurred())
	Expect(response).ToNot(BeNil())
	defer response.Body.Close()

	Expect(response.StatusCode).To(Equal(http.StatusOK))
	bodyBytes, err := io.ReadAll(response.Body)
	Expect(err).ToNot(HaveOccurred())

	var responseApp models.App
	err = json.Unmarshal(bodyBytes, &responseApp)
	Expect(err).ToNot(HaveOccurred(), string(bodyBytes))
	Expect(responseApp.Meta.Name).To(Equal(app))
	Expect(responseApp.Meta.Namespace).To(Equal(namespace))

	return responseApp
}

func appCreate(namespace string, body io.Reader) ([]byte, int) {
	GinkgoHelper()

	endpoint := makeEndpoint(v1.Routes.Path("AppCreate", namespace))

	response, err := env.Curl(http.MethodPost, endpoint, body)
	Expect(err).ToNot(HaveOccurred())
	Expect(response).ToNot(BeNil())
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	Expect(err).ToNot(HaveOccurred())

	return bodyBytes, response.StatusCode
}

func appUpdate(namespace, app string, body io.Reader) ([]byte, int) {
	GinkgoHelper()

	endpoint := makeEndpoint(v1.Routes.Path("AppUpdate", namespace, app))
	response, err := env.Curl(http.MethodPatch, endpoint, body)
	Expect(err).ToNot(HaveOccurred())
	Expect(response).ToNot(BeNil())
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	Expect(err).ToNot(HaveOccurred())

	return bodyBytes, response.StatusCode
}

func appValidateCV(namespace, app string) ([]byte, int) {
	GinkgoHelper()

	endpoint := makeEndpoint(v1.Routes.Path("AppValidateCV", namespace, app))
	response, err := env.Curl(http.MethodGet, endpoint, nil)
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

func makeEndpoint(path string) string {
	return fmt.Sprintf("%s%s/%s", serverURL, v1.Root, path)
}

func ExpectResponseToBeOK(bodyBytes []byte, statusCode int) {
	GinkgoHelper()

	Expect(statusCode).To(Equal(http.StatusOK))

	response := models.Response{}
	err := json.Unmarshal(bodyBytes, &response)
	Expect(err).ToNot(HaveOccurred())
	Expect(response).To(Equal(models.ResponseOK))
}

func ExpectBadRequestError(bodyBytes []byte, statusCode int, expectedErrorMsg string) {
	GinkgoHelper()

	Expect(statusCode).To(Equal(http.StatusBadRequest))

	errorResponse := toError(bodyBytes)
	Expect(errorResponse.Errors[0].Title).To(Equal(expectedErrorMsg))
}

func toError(bodyBytes []byte) apierrors.ErrorResponse {
	GinkgoHelper()

	var errorResponse apierrors.ErrorResponse
	err := json.Unmarshal(bodyBytes, &errorResponse)
	Expect(err).ToNot(HaveOccurred())

	return errorResponse
}
