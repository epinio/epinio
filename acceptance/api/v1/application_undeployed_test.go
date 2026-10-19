// Copyright © 2021 - 2026 SUSE LLC
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
	"github.com/epinio/epinio/acceptance/helpers/proc"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps Undeployed Endpoints", LApplication, func() {
	var namespace string

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	patchBuildFields := func(appName, buildStatus, imageURL string) {
		patch := fmt.Sprintf(
			`{"spec":{"buildstatus":%q,"imageurl":%q}}`,
			buildStatus, imageURL,
		)
		out, err := proc.Kubectl(
			"patch", "apps", appName,
			"--namespace", namespace,
			"--type", "merge",
			"-p", patch,
		)
		Expect(err).ToNot(HaveOccurred(), out)
	}

	createApp := func(name string) {
		bodyBytes, statusCode := appCreate(namespace, toJSON(models.ApplicationCreateRequest{
			Name: name,
			Configuration: models.ApplicationUpdateRequest{
				AppChart: "standard",
			},
		}))
		Expect(statusCode).To(Equal(http.StatusCreated), string(bodyBytes))
	}

	getApps := func(path string) models.AppList {
		response, err := env.Curl("GET", serverURL+v1.Root+"/"+path, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var apps models.AppList
		Expect(json.Unmarshal(bodyBytes, &apps)).To(Succeed())
		return apps
	}

	getPagedApps := func(path string) struct {
		Items      []models.App `json:"items"`
		Page       int          `json:"page"`
		PageSize   int          `json:"pageSize"`
		TotalItems int          `json:"totalItems"`
		TotalPages int          `json:"totalPages"`
	} {
		response, err := env.Curl("GET", serverURL+v1.Root+"/"+path, strings.NewReader(""))
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
		Expect(json.Unmarshal(bodyBytes, &paged)).To(Succeed())
		return paged
	}

	namesOf := func(apps models.AppList) []string {
		names := make([]string, 0, len(apps))
		for _, app := range apps {
			names = append(names, app.Meta.Name)
		}
		return names
	}

	It("lists never-deployed apps on AppsUndeployed and AllAppsUndeployed", func() {
		neverDeployed := catalog.NewAppName()
		rebuilt := catalog.NewAppName()
		createApp(neverDeployed)
		createApp(rebuilt)
		defer env.DeleteApp(neverDeployed)
		defer env.DeleteApp(rebuilt)

		patchBuildFields(neverDeployed, models.AppBuildStatusBuild, "")
		patchBuildFields(rebuilt, models.AppBuildStatusBuild, "registry.example/apps/rebuilt:abc")

		nsUndeployed := getApps(v1.Routes.Path("AppsUndeployed", namespace))
		Expect(namesOf(nsUndeployed)).To(ContainElement(neverDeployed))
		Expect(namesOf(nsUndeployed)).NotTo(ContainElement(rebuilt))

		allUndeployed := getApps(v1.Routes.Path("AllAppsUndeployed"))
		Expect(namesOf(allUndeployed)).To(ContainElement(neverDeployed))
		Expect(namesOf(allUndeployed)).NotTo(ContainElement(rebuilt))

		regular := getApps(v1.Routes.Path("Apps", namespace))
		Expect(namesOf(regular)).To(ContainElement(rebuilt))
		Expect(namesOf(regular)).NotTo(ContainElement(neverDeployed))
	})

	It("supports search and pagination on undeployed lists", func() {
		prefix := catalog.NewAppName()
		matching := prefix + "-match"
		other := catalog.NewAppName()
		createApp(matching)
		createApp(other)
		defer env.DeleteApp(matching)
		defer env.DeleteApp(other)

		patchBuildFields(matching, models.AppBuildStatusBuild, "")
		patchBuildFields(other, models.AppBuildStatusBuild, "")

		paged := getPagedApps(fmt.Sprintf(
			"%s?search=%s&page=1&pageSize=1",
			v1.Routes.Path("AppsUndeployed", namespace),
			prefix,
		))
		Expect(paged.Page).To(Equal(1))
		Expect(paged.PageSize).To(Equal(1))
		Expect(paged.TotalItems).To(Equal(1))
		Expect(paged.Items).To(HaveLen(1))
		Expect(paged.Items[0].Meta.Name).To(Equal(matching))

		allPaged := getPagedApps(fmt.Sprintf(
			"%s?search=%s&page=1&pageSize=10",
			v1.Routes.Path("AllAppsUndeployed"),
			prefix,
		))
		Expect(allPaged.TotalItems).To(BeNumerically(">=", 1))
		found := false
		for _, app := range allPaged.Items {
			if app.Meta.Name == matching {
				found = true
			}
		}
		Expect(found).To(BeTrue())
	})
})
