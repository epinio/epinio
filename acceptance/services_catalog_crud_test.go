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
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func catalogServicesURL(name string) string {
	base := strings.TrimSuffix(serverURL, "/") + "/api/v1/catalogservices"
	if name == "" {
		return base
	}
	return base + "/" + name
}

var _ = Describe("Catalog Service CRUD", Label("service"), func() {
	var serviceName string

	BeforeEach(func() {
		serviceName = catalog.NewTmpName("catalogsvc-")
	})

	AfterEach(func() {
		_, _ = env.Curl("DELETE", catalogServicesURL(serviceName), nil)
	})

	It("creates, reads, updates, and deletes a catalog service", func() {
		By("POST creates a new catalog service")
		createBody, _ := json.Marshal(models.CatalogServiceCreateRequest{
			Name:             serviceName,
			ShortDescription: "ephemeral test",
			Description:      "long description",
			HelmChart:        "noop",
			ChartVersion:     "0.0.1",
			HelmRepo: models.HelmRepoRequest{
				Name: "example",
				URL:  "https://charts.example.com",
			},
			SecretTypes: []string{"Opaque"},
		})

		createResp, createError := env.Curl(
			"POST",
			catalogServicesURL(""),
			bytes.NewReader(createBody),
		)
		Expect(createError).ToNot(HaveOccurred())
		decodeBody(createResp.Body, nil)
		Expect(createResp.StatusCode).To(Equal(http.StatusCreated))

		By("POST again returns 409 Conflict")
		dupResp, dupError := env.Curl(
			"POST",
			catalogServicesURL(""),
			bytes.NewReader(createBody),
		)
		Expect(dupError).ToNot(HaveOccurred())
		decodeBody(dupResp.Body, nil)
		Expect(dupResp.StatusCode).To(Equal(http.StatusConflict))

		By("GET returns the created catalog service")
		showResp, showError := env.Curl(
			"GET",
			catalogServicesURL(serviceName),
			nil,
		)
		Expect(showError).ToNot(HaveOccurred())
		var shown models.CatalogService
		decodeBody(showResp.Body, &shown)
		Expect(showResp.StatusCode).To(Equal(http.StatusOK))
		Expect(shown.Meta.Name).To(Equal(serviceName))
		Expect(shown.HelmChart).To(Equal("noop"))
		Expect(shown.SecretTypes).To(ContainElement("Opaque"))

		By("GET /catalogservices includes the new entry")
		listResp, listError := env.Curl("GET", catalogServicesURL(""), nil)
		Expect(listError).ToNot(HaveOccurred())
		var list []models.CatalogService
		decodeBody(listResp.Body, &list)
		Expect(listResp.StatusCode).To(Equal(http.StatusOK))
		names := []string{}
		for _, item := range list {
			names = append(names, item.Meta.Name)
		}
		Expect(names).To(ContainElement(serviceName))

		By("PATCH updates the short description")
		updateBody, _ := json.Marshal(models.CatalogServiceUpdateRequest{
			ShortDescription: "updated short",
		})
		updateResp, updateError := env.Curl(
			"PATCH",
			catalogServicesURL(serviceName),
			bytes.NewReader(updateBody),
		)
		Expect(updateError).ToNot(HaveOccurred())
		decodeBody(updateResp.Body, nil)
		Expect(updateResp.StatusCode).To(Equal(http.StatusOK))

		By("GET reflects the updated description but leaves chart untouched")
		afterResp, afterError := env.Curl(
			"GET",
			catalogServicesURL(serviceName),
			nil,
		)
		Expect(afterError).ToNot(HaveOccurred())
		var after models.CatalogService
		decodeBody(afterResp.Body, &after)
		Expect(after.ShortDescription).To(Equal("updated short"))
		Expect(after.HelmChart).To(Equal("noop"))

		By("DELETE removes the catalog service")
		deleteResp, deleteError := env.Curl(
			"DELETE",
			catalogServicesURL(serviceName),
			nil,
		)
		Expect(deleteError).ToNot(HaveOccurred())
		decodeBody(deleteResp.Body, nil)
		Expect(deleteResp.StatusCode).To(Equal(http.StatusOK))
	})

	It("rejects POST without required fields", func() {
		body, _ := json.Marshal(models.CatalogServiceCreateRequest{
			Name: serviceName,
			// chart missing
		})
		resp, err := env.Curl(
			"POST",
			catalogServicesURL(""),
			bytes.NewReader(body),
		)
		Expect(err).ToNot(HaveOccurred())
		decodeBody(resp.Body, nil)
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})

	It("returns 404 for PATCH/DELETE against an unknown name", func() {
		updateBody, _ := json.Marshal(models.CatalogServiceUpdateRequest{
			ShortDescription: "x",
		})
		patchResp, patchError := env.Curl(
			"PATCH",
			catalogServicesURL(serviceName+"-missing"),
			bytes.NewReader(updateBody),
		)
		Expect(patchError).ToNot(HaveOccurred())
		decodeBody(patchResp.Body, nil)
		Expect(patchResp.StatusCode).To(Equal(http.StatusNotFound))

		deleteResp, deleteError := env.Curl(
			"DELETE",
			catalogServicesURL(serviceName+"-missing"),
			nil,
		)
		Expect(deleteError).ToNot(HaveOccurred())
		decodeBody(deleteResp.Body, nil)
		Expect(deleteResp.StatusCode).To(Equal(http.StatusNotFound))
	})
})
