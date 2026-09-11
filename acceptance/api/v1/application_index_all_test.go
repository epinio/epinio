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
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AllApps Endpoints", LApplication, func() {
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

	It("lists all applications belonging to all namespaces", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/applications",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var apps models.AppList
		err = json.Unmarshal(bodyBytes, &apps)
		Expect(err).ToNot(HaveOccurred())

		// `apps` contains all apps. Not just the two we are looking for, from
		// the setup of this test. Everything which still exists from other
		// tests executing concurrently, or not cleaned by previous tests, or
		// the setup, or ... So, we cannot be sure that the two apps are in the
		// two first elements of the slice.

		var appRefs [][]string
		for _, a := range apps {
			appRefs = append(appRefs, []string{a.Meta.Name, a.Meta.Namespace})
		}
		Expect(appRefs).To(ContainElements(
			[]string{app1, namespace1},
			[]string{app2, namespace2}))
	})

	// listApps GETs /applications with the given query string and returns the
	// plain, non-paginated list.
	listApps := func(query string) models.AppList {
		url := fmt.Sprintf("%s%s/applications?%s", serverURL, v1.Root, query)

		response, err := env.Curl("GET", url, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var apps models.AppList
		err = json.Unmarshal(bodyBytes, &apps)
		Expect(err).ToNot(HaveOccurred(), string(bodyBytes))

		return apps
	}

	appRefsOf := func(apps models.AppList) [][]string {
		var appRefs [][]string
		for _, a := range apps {
			appRefs = append(appRefs, []string{a.Meta.Name, a.Meta.Namespace})
		}

		return appRefs
	}

	It("filters applications to a single namespace", func() {
		apps := listApps("namespaces=" + namespace1)

		Expect(appRefsOf(apps)).To(ConsistOf([]string{app1, namespace1}))
	})

	It("filters applications to several namespaces", func() {
		query := fmt.Sprintf("namespaces=%s,%s", namespace1, namespace2)

		apps := listApps(query)

		Expect(appRefsOf(apps)).To(ConsistOf(
			[]string{app1, namespace1},
			[]string{app2, namespace2}))
	})

	It("ignores a requested namespace that does not exist", func() {
		query := fmt.Sprintf("namespaces=%s,does-not-exist", namespace1)

		apps := listApps(query)

		Expect(appRefsOf(apps)).To(ConsistOf([]string{app1, namespace1}))
	})

	It("combines the namespace filter with the search term", func() {
		query := fmt.Sprintf("namespaces=%s,%s&search=%s",
			namespace1, namespace2, app1)

		apps := listApps(query)

		Expect(appRefsOf(apps)).To(ConsistOf([]string{app1, namespace1}))
	})

	It("paginates within the filtered namespaces", func() {
		url := fmt.Sprintf(
			"%s%s/applications?namespaces=%s,%s&page=1&pageSize=1",
			serverURL, v1.Root, namespace1, namespace2)

		response, err := env.Curl("GET", url, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var paged struct {
			Items      []models.App `json:"items"`
			Page       int          `json:"page"`
			PageSize   int          `json:"pageSize"`
			TotalItems int          `json:"totalItems"`
			TotalPages int          `json:"totalPages"`
		}
		err = json.Unmarshal(bodyBytes, &paged)
		Expect(err).ToNot(HaveOccurred(), string(bodyBytes))

		// The namespace filter makes the totals deterministic despite apps
		// left over from other specs running concurrently.
		Expect(paged.TotalItems).To(Equal(2))
		Expect(paged.TotalPages).To(Equal(2))
		Expect(paged.Items).To(HaveLen(1))
	})

	It("cannot widen access beyond the user's namespaces", func() {
		endpoint := fmt.Sprintf("%s%s/applications?namespaces=%s",
			serverURL, v1.Root, namespace1)
		request, err := http.NewRequest(http.MethodGet, endpoint, nil)
		Expect(err).ToNot(HaveOccurred())
		request.SetBasicAuth(user, password)

		response, err := env.Client().Do(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var apps models.AppList
		err = json.Unmarshal(bodyBytes, &apps)
		Expect(err).ToNot(HaveOccurred())
		Expect(apps).To(BeEmpty())
	})

	It("doesn't list applications belonging to non-accessible namespaces", func() {
		endpoint := fmt.Sprintf("%s%s/applications", serverURL, v1.Root)
		request, err := http.NewRequest(http.MethodGet, endpoint, nil)
		Expect(err).ToNot(HaveOccurred())
		request.SetBasicAuth(user, password)

		response, err := env.Client().Do(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var apps models.AppList
		err = json.Unmarshal(bodyBytes, &apps)
		Expect(err).ToNot(HaveOccurred())
		Expect(apps).To(BeEmpty())
	})
})
