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
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AllAppsGrouped Endpoint", LApplication, func() {
	var (
		namespace1, namespace2 string
		app1, app2             string
		user, password         string
		containerImageURL      string
	)

	BeforeEach(func() {
		containerImageURL = "epinio/sample-app"

		namespace1 = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace1)
		app1 = catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)

		namespace2 = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace2)
		app2 = catalog.NewAppName()
		env.MakeContainerImageApp(app2, 1, containerImageURL)

		user, password = env.CreateEpinioUser("user", nil)
	})

	AfterEach(func() {
		env.TargetNamespace(namespace2)
		env.DeleteApp(app2)

		env.TargetNamespace(namespace1)
		env.DeleteApp(app1)

		env.DeleteNamespace(namespace1)
		env.DeleteNamespace(namespace2)

		env.DeleteEpinioUser(user)
	})

	groupedURL := func(page, pageSize int) string {
		return fmt.Sprintf("%s%s/applications/grouped?page=%d&pageSize=%d",
			serverURL, v1.Root, page, pageSize)
	}

	parseGrouped := func(body []byte) map[string]response.PaginatedResponse[models.App] {
		GinkgoHelper()
		var result map[string]response.PaginatedResponse[models.App]
		Expect(json.Unmarshal(body, &result)).To(Succeed())
		return result
	}

	It("returns one entry per namespace keyed by namespace name", func() {
		resp, err := env.Curl("GET", groupedURL(1, 10), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK), string(body))

		grouped := parseGrouped(body)
		Expect(grouped).To(HaveKey(namespace1))
		Expect(grouped).To(HaveKey(namespace2))
	})

	It("includes the correct app in each namespace bucket", func() {
		resp, err := env.Curl("GET", groupedURL(1, 10), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK), string(body))

		grouped := parseGrouped(body)

		ns1Apps := grouped[namespace1].Items
		Expect(ns1Apps).To(HaveLen(1))
		Expect(ns1Apps[0].Meta.Name).To(Equal(app1))
		Expect(ns1Apps[0].Meta.Namespace).To(Equal(namespace1))

		ns2Apps := grouped[namespace2].Items
		Expect(ns2Apps).To(HaveLen(1))
		Expect(ns2Apps[0].Meta.Name).To(Equal(app2))
		Expect(ns2Apps[0].Meta.Namespace).To(Equal(namespace2))
	})

	It("returns correct pagination metadata per namespace", func() {
		resp, err := env.Curl("GET", groupedURL(1, 10), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK), string(body))

		grouped := parseGrouped(body)

		for _, ns := range []string{namespace1, namespace2} {
			meta := grouped[ns]
			Expect(meta.Page).To(Equal(1))
			Expect(meta.PageSize).To(Equal(10))
			Expect(meta.TotalItems).To(Equal(1))
			Expect(meta.TotalPages).To(Equal(1))
		}
	})

	It("returns empty items for a namespace on a page beyond total", func() {
		// page 2 with pageSize 10 for a namespace with 1 app → empty items
		resp, err := env.Curl("GET", groupedURL(2, 10), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK), string(body))

		grouped := parseGrouped(body)
		Expect(grouped[namespace1].Items).To(BeEmpty())
		Expect(grouped[namespace2].Items).To(BeEmpty())
	})

	It("returns no apps for a user without namespace access", func() {
		endpoint := groupedURL(1, 10)
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		Expect(err).ToNot(HaveOccurred())
		req.SetBasicAuth(user, password)

		resp, err := env.Client().Do(req)
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK), string(body))

		grouped := parseGrouped(body)
		for _, page := range grouped {
			Expect(page.Items).To(BeEmpty())
		}
	})

	It("filters apps by search term within each namespace", func() {
		url := fmt.Sprintf("%s%s/applications/grouped?page=1&pageSize=10&search=%s",
			serverURL, v1.Root, app1)

		resp, err := env.Curl("GET", url, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK), string(body))

		grouped := parseGrouped(body)

		// namespace1 has app1 which matches the search
		Expect(grouped[namespace1].Items).To(HaveLen(1))
		Expect(grouped[namespace1].Items[0].Meta.Name).To(Equal(app1))

		// namespace2 has app2 which does not match
		Expect(grouped[namespace2].Items).To(BeEmpty())
	})
})
