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

var _ = Describe("Apps Endpoint", LApplication, func() {
	var (
		namespace string
	)
	containerImageURL := "epinio/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	It("lists all applications belonging to the namespace", func() {
		app1 := catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)
		defer env.DeleteApp(app1)
		app2 := catalog.NewAppName()
		env.MakeContainerImageApp(app2, 1, containerImageURL)
		defer env.DeleteApp(app2)

		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications",
			serverURL, v1.Root, namespace), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var apps models.AppList
		err = json.Unmarshal(bodyBytes, &apps)
		Expect(err).ToNot(HaveOccurred())

		appNames := []string{apps[0].Meta.Name, apps[1].Meta.Name}
		Expect(appNames).To(ContainElements(app1, app2))

		namespaceNames := []string{apps[0].Meta.Namespace, apps[1].Meta.Namespace}
		Expect(namespaceNames).To(ContainElements(namespace, namespace))

		// Applications are deployed. Must have workload.
		statuses := []string{apps[0].Workload.Status, apps[1].Workload.Status}
		Expect(statuses).To(ContainElements("1/1", "1/1"))

		// App is in "running" status
		Expect(apps[0].Status).To(BeEquivalentTo("running"))
	})

	It("returns a paginated response when page params are provided", func() {
		app1 := catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)
		defer env.DeleteApp(app1)
		app2 := catalog.NewAppName()
		env.MakeContainerImageApp(app2, 1, containerImageURL)
		defer env.DeleteApp(app2)

		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications?page=1&pageSize=1",
			serverURL, v1.Root, namespace), strings.NewReader(""))
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

		Expect(paged.Page).To(Equal(1))
		Expect(paged.PageSize).To(Equal(1))
		Expect(paged.TotalItems).To(Equal(2))
		Expect(paged.TotalPages).To(Equal(2))
		Expect(paged.Items).To(HaveLen(1))
	})

	It("filters applications by search term", func() {
		app1 := catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)
		defer env.DeleteApp(app1)
		app2 := catalog.NewAppName()
		env.MakeContainerImageApp(app2, 1, containerImageURL)
		defer env.DeleteApp(app2)

		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications?search=%s",
			serverURL, v1.Root, namespace, app1), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var paged struct {
			Items []models.App `json:"items"`
		}
		err = json.Unmarshal(bodyBytes, &paged)
		Expect(err).ToNot(HaveOccurred(), string(bodyBytes))

		Expect(paged.Items).ToNot(BeEmpty())
		for _, app := range paged.Items {
			Expect(strings.Contains(app.Meta.Name, app1)).To(BeTrue(),
				"expected app name %q to contain search term %q", app.Meta.Name, app1)
		}
		names := make([]string, 0, len(paged.Items))
		for _, app := range paged.Items {
			names = append(names, app.Meta.Name)
		}
		Expect(names).To(ContainElement(app1))
	})

	It("returns a 404 when the namespace does not exist", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/idontexist/applications",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})
})
