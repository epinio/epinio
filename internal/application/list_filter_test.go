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
		It("is true for create or build with no deployed image", func() {
			Expect(application.IsUndeployed(models.App{
				BuildStatus: models.AppBuildStatusCreate,
			})).To(BeTrue())
			Expect(application.IsUndeployed(models.App{
				BuildStatus: models.AppBuildStatusBuild,
			})).To(BeTrue())
			Expect(application.IsUndeployed(models.App{
				BuildStatus: models.AppBuildStatusBuild,
				ImageURL:    "registry.example/apps/ns-app:abc",
			})).To(BeFalse())
			Expect(application.IsUndeployed(models.App{
				BuildStatus: models.AppBuildStatusDeployed,
				ImageURL:    "registry.example/apps/ns-app:abc",
			})).To(BeFalse())
			Expect(application.IsUndeployed(models.App{})).To(BeFalse())
		})
	})

	Describe("IsUndeployedCR", func() {
		It("requires create or build status and empty imageurl", func() {
			created := &unstructured.Unstructured{Object: map[string]any{
				"spec": map[string]any{"buildstatus": models.AppBuildStatusCreate},
			}}
			neverDeployed := &unstructured.Unstructured{Object: map[string]any{
				"spec": map[string]any{"buildstatus": models.AppBuildStatusBuild},
			}}
			rebuiltDeployed := &unstructured.Unstructured{Object: map[string]any{
				"spec": map[string]any{
					"buildstatus": models.AppBuildStatusBuild,
					"imageurl":    "registry.example/apps/ns-app:abc",
				},
			}}
			deployed := &unstructured.Unstructured{Object: map[string]any{
				"spec": map[string]any{
					"buildstatus": models.AppBuildStatusDeployed,
					"imageurl":    "registry.example/apps/ns-app:abc",
				},
			}}
			empty := &unstructured.Unstructured{Object: map[string]any{}}

			Expect(application.IsUndeployedCR(created)).To(BeTrue())
			Expect(application.IsUndeployedCR(neverDeployed)).To(BeTrue())
			Expect(application.IsUndeployedCR(rebuiltDeployed)).To(BeFalse())
			Expect(application.IsUndeployedCR(deployed)).To(BeFalse())
			Expect(application.IsUndeployedCR(empty)).To(BeFalse())
		})
	})

	Describe("FilterApps", func() {
		var apps models.AppList

		BeforeEach(func() {
			apps = models.AppList{
				{Meta: models.NewAppRef("created", "ns"), BuildStatus: models.AppBuildStatusCreate},
				{Meta: models.NewAppRef("built", "ns"), BuildStatus: models.AppBuildStatusBuild},
				{
					Meta:        models.NewAppRef("rebuilt", "ns"),
					BuildStatus: models.AppBuildStatusBuild,
					ImageURL:    "registry.example/apps/ns-rebuilt:abc",
				},
				{
					Meta:        models.NewAppRef("running", "ns"),
					BuildStatus: models.AppBuildStatusDeployed,
					ImageURL:    "registry.example/apps/ns-running:abc",
				},
			}
		})

		It("keeps everything for ListFilterAll", func() {
			Expect(application.FilterApps(apps, application.ListFilterAll)).To(HaveLen(4))
		})

		It("keeps deployed and previously deployed apps for ListFilterDeployed", func() {
			filtered := application.FilterApps(apps, application.ListFilterDeployed)
			Expect(filtered).To(HaveLen(2))
			Expect(filtered[0].Meta.Name).To(Equal("rebuilt"))
			Expect(filtered[1].Meta.Name).To(Equal("running"))
		})

		It("keeps only never-deployed apps for ListFilterUndeployed", func() {
			filtered := application.FilterApps(apps, application.ListFilterUndeployed)
			Expect(filtered).To(HaveLen(2))
			Expect(filtered[0].Meta.Name).To(Equal("created"))
			Expect(filtered[1].Meta.Name).To(Equal("built"))
		})

		It("keeps a deployed app that was later rebuilt build-only in the regular list", func() {
			filtered := application.FilterApps(apps, application.ListFilterDeployed)
			names := []string{filtered[0].Meta.Name, filtered[1].Meta.Name}
			Expect(names).To(ContainElement("rebuilt"))
			Expect(application.FilterApps(apps, application.ListFilterUndeployed)).NotTo(
				ContainElement(HaveField("Meta.Name", "rebuilt")),
			)
		})
	})

	Describe("PageAppCRs", func() {
		It("returns an empty page without clamping when page is out of range", func() {
			crs := []unstructured.Unstructured{
				{Object: map[string]any{"metadata": map[string]any{"name": "a"}}},
				{Object: map[string]any{"metadata": map[string]any{"name": "b"}}},
			}
			page, total := application.PageAppCRs(crs, 99, 1)
			Expect(total).To(Equal(2))
			Expect(page).To(BeEmpty())
		})
	})

	Describe("FilterAppCRs", func() {
		It("applies search and undeployed filter together", func() {
			crs := []unstructured.Unstructured{
				{Object: map[string]any{
					"metadata": map[string]any{"name": "built-alpha"},
					"spec":     map[string]any{"buildstatus": models.AppBuildStatusBuild},
				}},
				{Object: map[string]any{
					"metadata": map[string]any{"name": "built-beta"},
					"spec": map[string]any{
						"buildstatus": models.AppBuildStatusBuild,
						"imageurl":    "registry.example/apps/beta:1",
					},
				}},
				{Object: map[string]any{
					"metadata": map[string]any{"name": "other"},
					"spec":     map[string]any{"buildstatus": models.AppBuildStatusBuild},
				}},
			}

			matched := application.FilterAppCRs(crs, "built", application.ListFilterUndeployed)
			Expect(matched).To(HaveLen(1))
			Expect(matched[0].GetName()).To(Equal("built-alpha"))
		})
	})
})
