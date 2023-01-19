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
	"encoding/json"
	"io"
	"net/http"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/testenv"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppUpload Endpoint", LApplication, func() {
	var (
		namespace string
		url       string
		path      string
		request   *http.Request
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	JustBeforeEach(func() {
		url = serverURL + v1.Root + "/" + v1.Routes.Path("AppUpload", namespace, "testapp")
		var err error
		request, err = uploadRequest(url, path)
		Expect(err).ToNot(HaveOccurred())
	})

	When("uploading a tar file", func() {
		BeforeEach(func() {
			path = testenv.TestAssetPath("sample-app.tar")
		})

		It("returns the app response", func() {
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			r := &models.UploadResponse{}
			err = json.Unmarshal(bodyBytes, &r)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.BlobUID).ToNot(BeEmpty())
		})
	})

	When("uploading a tgz (gzip) file", func() {
		BeforeEach(func() {
			path = testenv.TestAssetPath("sample-app.tgz")
		})

		It("returns the app response", func() {
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			r := &models.UploadResponse{}
			err = json.Unmarshal(bodyBytes, &r)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.BlobUID).ToNot(BeEmpty())
		})
	})

	When("uploading a txz (xz) file", func() {
		BeforeEach(func() {
			path = testenv.TestAssetPath("sample-app.txz")
		})

		It("returns the app response", func() {
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			r := &models.UploadResponse{}
			err = json.Unmarshal(bodyBytes, &r)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.BlobUID).ToNot(BeEmpty())
		})
	})

	When("uploading a tbz (bz2) file", func() {
		BeforeEach(func() {
			path = testenv.TestAssetPath("sample-app.tbz")
		})

		It("returns the app response", func() {
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			r := &models.UploadResponse{}
			err = json.Unmarshal(bodyBytes, &r)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.BlobUID).ToNot(BeEmpty())
		})
	})

	When("uploading a zip file", func() {
		BeforeEach(func() {
			path = testenv.TestAssetPath("sample-app.zip")
		})

		It("returns the app response", func() {
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			r := &models.UploadResponse{}
			err = json.Unmarshal(bodyBytes, &r)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.BlobUID).ToNot(BeEmpty())
		})
	})

	When("uploading a non-supported archive type", func() {
		BeforeEach(func() {
			path = testenv.TestAssetPath("sample-app.rar")
		})

		It("returns the app response", func() {
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))

			var responseBody map[string][]errors.APIError
			json.Unmarshal(bodyBytes, &responseBody)
			Expect(responseBody["errors"][0].Title).To(Equal("archive type not supported [application/vnd.rar]"))
		})
	})
})
