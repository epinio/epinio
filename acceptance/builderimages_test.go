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

package acceptance_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// builderImagesURL returns the API URL for the builderimages collection or
// a single named entry.
func builderImagesURL(name string) string {
	base := strings.TrimSuffix(serverURL, "/") + "/api/v1/builderimages"
	if name == "" {
		return base
	}
	return base + "/" + name
}

func decodeBody(body io.ReadCloser, target interface{}) {
	defer body.Close()
	raw, readError := io.ReadAll(body)
	Expect(readError).ToNot(HaveOccurred())
	if target == nil {
		return
	}
	Expect(json.Unmarshal(raw, target)).To(Succeed())
}

var _ = Describe("Builder Image CRUD", Label("builderimage"), func() {
	var imageName string

	BeforeEach(func() {
		imageName = catalog.NewTmpName("builderimage-")
	})

	AfterEach(func() {
		// Best-effort cleanup. Ignore errors — name may not exist if
		// the test deleted it already or never created it.
		_, _ = env.Curl("DELETE", builderImagesURL(imageName), nil)
	})

	It("creates, reads, updates, and deletes a builder image", func() {
		By("POST creates a new builder image")
		createBody, marshalError := json.Marshal(models.BuilderImageCreateRequest{
			Name:             imageName,
			Image:            "registry.example.com/builder:latest",
			ShortDescription: "test builder",
			Description:      "long form description",
		})
		Expect(marshalError).ToNot(HaveOccurred())

		createResp, createError := env.Curl(
			"POST",
			builderImagesURL(""),
			bytes.NewReader(createBody),
		)
		Expect(createError).ToNot(HaveOccurred())
		decodeBody(createResp.Body, nil)
		Expect(createResp.StatusCode).To(Equal(http.StatusCreated))

		By("POST again returns 409 Conflict")
		conflictResp, conflictError := env.Curl(
			"POST",
			builderImagesURL(""),
			bytes.NewReader(createBody),
		)
		Expect(conflictError).ToNot(HaveOccurred())
		decodeBody(conflictResp.Body, nil)
		Expect(conflictResp.StatusCode).To(Equal(http.StatusConflict))

		By("GET returns the created builder image")
		showResp, showError := env.Curl("GET", builderImagesURL(imageName), nil)
		Expect(showError).ToNot(HaveOccurred())
		var shown models.BuilderImage
		decodeBody(showResp.Body, &shown)
		Expect(showResp.StatusCode).To(Equal(http.StatusOK))
		Expect(shown.Meta.Name).To(Equal(imageName))
		Expect(shown.Image).To(Equal("registry.example.com/builder:latest"))
		Expect(shown.ShortDescription).To(Equal("test builder"))

		By("GET /builderimages includes the new entry")
		listResp, listError := env.Curl("GET", builderImagesURL(""), nil)
		Expect(listError).ToNot(HaveOccurred())
		var list models.BuilderImageList
		decodeBody(listResp.Body, &list)
		Expect(listResp.StatusCode).To(Equal(http.StatusOK))
		names := []string{}
		for _, item := range list {
			names = append(names, item.Meta.Name)
		}
		Expect(names).To(ContainElement(imageName))

		By("PATCH updates the description")
		updateBody, _ := json.Marshal(models.BuilderImageUpdateRequest{
			ShortDescription: "updated short",
		})
		updateResp, updateError := env.Curl(
			"PATCH",
			builderImagesURL(imageName),
			bytes.NewReader(updateBody),
		)
		Expect(updateError).ToNot(HaveOccurred())
		decodeBody(updateResp.Body, nil)
		Expect(updateResp.StatusCode).To(Equal(http.StatusOK))

		By("GET reflects the updated description")
		afterResp, afterError := env.Curl("GET", builderImagesURL(imageName), nil)
		Expect(afterError).ToNot(HaveOccurred())
		var after models.BuilderImage
		decodeBody(afterResp.Body, &after)
		Expect(after.ShortDescription).To(Equal("updated short"))
		// Image untouched by partial update
		Expect(after.Image).To(Equal("registry.example.com/builder:latest"))

		By("DELETE removes the builder image")
		deleteResp, deleteError := env.Curl(
			"DELETE",
			builderImagesURL(imageName),
			nil,
		)
		Expect(deleteError).ToNot(HaveOccurred())
		decodeBody(deleteResp.Body, nil)
		Expect(deleteResp.StatusCode).To(Equal(http.StatusOK))

		By("GET after delete returns 404")
		gone, goneError := env.Curl("GET", builderImagesURL(imageName), nil)
		Expect(goneError).ToNot(HaveOccurred())
		decodeBody(gone.Body, nil)
		Expect(gone.StatusCode).To(Equal(http.StatusNotFound))
	})

	It("rejects POST without required fields", func() {
		body, _ := json.Marshal(models.BuilderImageCreateRequest{Name: "x"})
		resp, err := env.Curl(
			"POST",
			builderImagesURL(""),
			bytes.NewReader(body),
		)
		Expect(err).ToNot(HaveOccurred())
		decodeBody(resp.Body, nil)
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})

	It("returns 404 for PATCH against an unknown name", func() {
		body, _ := json.Marshal(models.BuilderImageUpdateRequest{
			ShortDescription: "x",
		})
		resp, err := env.Curl(
			"PATCH",
			builderImagesURL(fmt.Sprintf("%s-missing", imageName)),
			bytes.NewReader(body),
		)
		Expect(err).ToNot(HaveOccurred())
		decodeBody(resp.Body, nil)
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
	})
})
