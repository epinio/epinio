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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppSource Endpoint", LApplication, func() {
	var (
		namespace string
		appName   string
	)

	defaultBuilder := "paketobuildpacks/builder-jammy-full:0.3.290"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()

		appCreateRequest := models.ApplicationCreateRequest{Name: appName}
		bodyBytes, statusCode := appCreate(namespace, toJSON(appCreateRequest))
		Expect(statusCode).To(Equal(http.StatusCreated), string(bodyBytes))
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	sourceURL := func(ns, app string) string {
		return fmt.Sprintf("%s%s/namespaces/%s/applications/%s/source",
			serverURL, v1.Root, ns, app)
	}

	When("the application has a staged source blob", func() {
		BeforeEach(func() {
			uploadResponse := uploadApplication(appName, namespace)
			stageRequest := models.StageRequest{
				App: models.AppRef{
					Meta: models.Meta{
						Name:      appName,
						Namespace: namespace,
					},
				},
				BlobUID:      uploadResponse.BlobUID,
				BuilderImage: defaultBuilder,
			}
			stageApplication(appName, namespace, stageRequest)
		})

		It("returns the source tarball", func() {
			response, err := env.Curl("GET", sourceURL(namespace, appName), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()

			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
			Expect(len(bodyBytes)).To(BeNumerically(">", 0))

			tarReader := tar.NewReader(bytes.NewReader(bodyBytes))
			_, err = tarReader.Next()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("the application has no stored source", func() {
		It("returns a bad request error", func() {
			response, err := env.Curl("GET", sourceURL(namespace, appName), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			defer response.Body.Close()

			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))

			errResponse := &apierrors.ErrorResponse{}
			err = json.Unmarshal(bodyBytes, errResponse)
			Expect(err).ToNot(HaveOccurred())
			Expect(errResponse.Errors).To(HaveLen(1))
			Expect(errResponse.Errors[0].Title).To(Equal("application has no stored source"))
		})
	})

	When("the application does not exist", func() {
		It("returns a not found error", func() {
			response, err := env.Curl("GET", sourceURL(namespace, "bogus"), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			defer response.Body.Close()

			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})
})
