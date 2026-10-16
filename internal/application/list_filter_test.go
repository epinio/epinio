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

package application_test

import (
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("ListFilter helpers", func() {
	Describe("IsUndeployed", func() {
		It("is true only for buildstatus build", func() {
			Expect(application.IsUndeployed(models.App{BuildStatus: models.AppBuildStatusBuild})).To(BeTrue())
			Expect(application.IsUndeployed(models.App{BuildStatus: models.AppBuildStatusDeployed})).To(BeFalse())
			Expect(application.IsUndeployed(models.App{})).To(BeFalse())
		})
	})

	Describe("IsUndeployedCR", func() {
		It("reads spec.buildstatus from the CR", func() {
			undeployed := &unstructured.Unstructured{Object: map[string]any{
				"spec": map[string]any{"buildstatus": models.AppBuildStatusBuild},
			}}
			deployed := &unstructured.Unstructured{Object: map[string]any{
				"spec": map[string]any{"buildstatus": models.AppBuildStatusDeployed},
			}}
			empty := &unstructured.Unstructured{Object: map[string]any{}}

			Expect(application.IsUndeployedCR(undeployed)).To(BeTrue())
			Expect(application.IsUndeployedCR(deployed)).To(BeFalse())
			Expect(application.IsUndeployedCR(empty)).To(BeFalse())
		})
	})

	Describe("FilterApps", func() {
		var apps models.AppList

		BeforeEach(func() {
			apps = models.AppList{
				{Meta: models.NewAppRef("built", "ns"), BuildStatus: models.AppBuildStatusBuild},
				{Meta: models.NewAppRef("running", "ns"), BuildStatus: models.AppBuildStatusDeployed},
				{Meta: models.NewAppRef("created", "ns")},
			}
		})

		It("keeps everything for ListFilterAll", func() {
			Expect(application.FilterApps(apps, application.ListFilterAll)).To(HaveLen(3))
		})

		It("drops build-only apps for ListFilterDeployed", func() {
			filtered := application.FilterApps(apps, application.ListFilterDeployed)
			Expect(filtered).To(HaveLen(2))
			Expect(filtered[0].Meta.Name).To(Equal("running"))
			Expect(filtered[1].Meta.Name).To(Equal("created"))
		})

		It("keeps only build-only apps for ListFilterUndeployed", func() {
			filtered := application.FilterApps(apps, application.ListFilterUndeployed)
			Expect(filtered).To(HaveLen(1))
			Expect(filtered[0].Meta.Name).To(Equal("built"))
		})
	})
})
