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

package appchart_test

import (
	"context"
	"errors"

	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/testfakes/k8sdynamic/k8sdynamicfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("AppChart CRUD", func() {
	var (
		ctx    context.Context
		fakeRI *k8sdynamicfakes.FakeResourceInterface
		fakeNS *k8sdynamicfakes.FakeNamespaceableResourceInterface
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeRI = &k8sdynamicfakes.FakeResourceInterface{}
		fakeNS = &k8sdynamicfakes.FakeNamespaceableResourceInterface{}
		fakeNS.NamespaceReturns(fakeRI)
	})

	makeCR := func(name string) unstructured.Unstructured {
		return unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{"name": name},
				"spec": map[string]interface{}{
					"description":      "long",
					"shortDescription": "short",
					"helmChart":        "https://example.com/chart.tgz",
					"helmRepo":         "https://example.com",
					"values": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		}
	}

	Describe("List", func() {
		It("returns all known appcharts", func() {
			a := makeCR("standard")
			b := makeCR("custom")
			fakeRI.ListReturns(&unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{a, b},
			}, nil)

			items, listError := appchart.List(ctx, fakeNS)
			Expect(listError).ToNot(HaveOccurred())
			Expect(items).To(HaveLen(2))
			Expect(items[0].Meta.Name).To(Equal("standard"))
			Expect(items[1].Meta.Name).To(Equal("custom"))
		})

		It("propagates list errors", func() {
			fakeRI.ListReturns(nil, errors.New("boom"))
			_, listError := appchart.List(ctx, fakeNS)
			Expect(listError).To(HaveOccurred())
		})
	})

	Describe("Exists / Lookup", func() {
		It("Exists returns false on NotFound", func() {
			fakeRI.GetReturns(
				nil,
				k8sapierrors.NewNotFound(
					schema.GroupResource{Resource: "appcharts"},
					"missing",
				),
			)
			exists, existsError := appchart.Exists(ctx, fakeNS, "missing")
			Expect(existsError).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("Lookup returns nil for not-found", func() {
			fakeRI.GetReturns(
				nil,
				k8sapierrors.NewNotFound(
					schema.GroupResource{Resource: "appcharts"},
					"missing",
				),
			)
			found, lookupError := appchart.Lookup(ctx, fakeNS, "missing")
			Expect(lookupError).ToNot(HaveOccurred())
			Expect(found).To(BeNil())
		})

		It("Lookup returns the AppChartFull for an existing chart", func() {
			cr := makeCR("standard")
			fakeRI.GetReturns(&cr, nil)

			found, lookupError := appchart.Lookup(ctx, fakeNS, "standard")
			Expect(lookupError).ToNot(HaveOccurred())
			Expect(found).ToNot(BeNil())
			Expect(found.AppChart.Meta.Name).To(Equal("standard"))
			Expect(found.AppChart.HelmChart).To(Equal("https://example.com/chart.tgz"))
			Expect(found.Values).To(Equal(map[string]string{"foo": "bar"}))
		})
	})

	Describe("Create", func() {
		It("sets Kind, APIVersion, Name, and management labels", func() {
			fakeRI.CreateReturns(&unstructured.Unstructured{}, nil)

			req := models.AppChartCreateRequest{
				Name:             "custom",
				HelmChart:        "https://example.com/custom.tgz",
				ShortDescription: "short",
				Description:      "long",
			}

			_, createError := appchart.Create(ctx, fakeNS, req)
			Expect(createError).ToNot(HaveOccurred())
			Expect(fakeRI.CreateCallCount()).To(Equal(1))

			_, sent, _, _ := fakeRI.CreateArgsForCall(0)
			Expect(sent.GetKind()).To(Equal("AppChart"))
			Expect(sent.GetAPIVersion()).To(Equal("application.epinio.io/v1"))
			Expect(sent.GetName()).To(Equal("custom"))
			Expect(
				sent.GetLabels()["app.kubernetes.io/managed-by"],
			).To(Equal("epinio"))
		})
	})

	Describe("Update", func() {
		It("merges only non-empty fields into the spec", func() {
			existing := makeCR("standard")
			fakeRI.GetReturns(&existing, nil)
			fakeRI.UpdateReturns(&unstructured.Unstructured{}, nil)

			req := models.AppChartUpdateRequest{
				ShortDescription: "new short",
			}
			updateError := appchart.Update(ctx, fakeNS, "standard", req)
			Expect(updateError).ToNot(HaveOccurred())

			_, sent, _, _ := fakeRI.UpdateArgsForCall(0)
			spec, _, _ := unstructured.NestedMap(sent.Object, "spec")
			Expect(spec["shortDescription"]).To(Equal("new short"))
			Expect(spec["description"]).To(Equal("long"))
			Expect(spec["helmChart"]).To(Equal("https://example.com/chart.tgz"))
		})

		It("replaces values when Values is non-nil", func() {
			existing := makeCR("standard")
			fakeRI.GetReturns(&existing, nil)
			fakeRI.UpdateReturns(&unstructured.Unstructured{}, nil)

			req := models.AppChartUpdateRequest{
				Values: map[string]string{"baz": "qux"},
			}
			updateError := appchart.Update(ctx, fakeNS, "standard", req)
			Expect(updateError).ToNot(HaveOccurred())

			_, sent, _, _ := fakeRI.UpdateArgsForCall(0)
			values, _, _ := unstructured.NestedStringMap(
				sent.Object,
				"spec",
				"values",
			)
			Expect(values).To(Equal(map[string]string{"baz": "qux"}))
		})
	})

	Describe("Delete", func() {
		It("issues a delete to the dynamic client", func() {
			fakeRI.DeleteReturns(nil)
			deleteError := appchart.Delete(ctx, fakeNS, "standard")
			Expect(deleteError).ToNot(HaveOccurred())
			Expect(fakeRI.DeleteCallCount()).To(Equal(1))

			_, name, _, _ := fakeRI.DeleteArgsForCall(0)
			Expect(name).To(Equal("standard"))
		})
	})
})
