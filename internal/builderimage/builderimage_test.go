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

package builderimage_test

import (
	"context"
	"errors"

	"github.com/epinio/epinio/internal/builderimage"
	"github.com/epinio/epinio/internal/testfakes/k8sdynamic/k8sdynamicfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("BuilderImage CRUD", func() {
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

	makeCR := func(name, image, desc, short string) unstructured.Unstructured {
		return unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{"name": name},
				"spec": map[string]interface{}{
					"image":            image,
					"description":      desc,
					"shortDescription": short,
				},
			},
		}
	}

	Describe("List", func() {
		It("returns all known builder images", func() {
			fakeRI.ListReturns(&unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					makeCR("standard", "paketo:jammy", "long", "short"),
					makeCR("alpha", "alpha:1.0", "", ""),
				},
			}, nil)

			items, listError := builderimage.List(ctx, fakeNS)

			Expect(listError).ToNot(HaveOccurred())
			Expect(items).To(HaveLen(2))
			Expect(items[0].Meta.Name).To(Equal("standard"))
			Expect(items[0].Image).To(Equal("paketo:jammy"))
			Expect(items[0].Description).To(Equal("long"))
			Expect(items[0].ShortDescription).To(Equal("short"))
		})

		It("propagates list errors", func() {
			fakeRI.ListReturns(
				nil,
				errors.New("boom"),
			)
			_, listError := builderimage.List(ctx, fakeNS)
			Expect(listError).To(HaveOccurred())
		})
	})

	Describe("Exists", func() {
		It("returns true when found", func() {
			cr := makeCR("standard", "img", "", "")
			fakeRI.GetReturns(&cr, nil)

			exists, existsError := builderimage.Exists(ctx, fakeNS, "standard")
			Expect(existsError).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("returns false when not found", func() {
			fakeRI.GetReturns(
				nil,
				k8sapierrors.NewNotFound(
					schema.GroupResource{Resource: "builderimages"},
					"missing",
				),
			)
			exists, existsError := builderimage.Exists(ctx, fakeNS, "missing")
			Expect(existsError).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})

	Describe("Lookup", func() {
		It("returns nil for not-found instead of an error", func() {
			fakeRI.GetReturns(
				nil,
				k8sapierrors.NewNotFound(
					schema.GroupResource{Resource: "builderimages"},
					"missing",
				),
			)

			found, lookupError := builderimage.Lookup(ctx, fakeNS, "missing")
			Expect(lookupError).ToNot(HaveOccurred())
			Expect(found).To(BeNil())
		})

		It("returns the converted CR when found", func() {
			cr := makeCR("standard", "paketo:jammy", "long", "short")
			fakeRI.GetReturns(&cr, nil)

			found, lookupError := builderimage.Lookup(ctx, fakeNS, "standard")
			Expect(lookupError).ToNot(HaveOccurred())
			Expect(found).ToNot(BeNil())
			Expect(found.Meta.Name).To(Equal("standard"))
			Expect(found.Image).To(Equal("paketo:jammy"))
		})
	})

	Describe("Create", func() {
		It("sets Kind, APIVersion, Name, and management labels", func() {
			fakeRI.CreateReturns(&unstructured.Unstructured{}, nil)

			req := models.BuilderImageCreateRequest{
				Name:             "custom",
				Image:            "registry.example.com/builder:latest",
				Description:      "long",
				ShortDescription: "short",
			}

			_, createError := builderimage.Create(ctx, fakeNS, req)
			Expect(createError).ToNot(HaveOccurred())
			Expect(fakeRI.CreateCallCount()).To(Equal(1))

			_, sent, _, _ := fakeRI.CreateArgsForCall(0)
			Expect(sent.GetKind()).To(Equal("BuilderImage"))
			Expect(sent.GetAPIVersion()).To(Equal("application.epinio.io/v1"))
			Expect(sent.GetName()).To(Equal("custom"))
			Expect(
				sent.GetLabels()["app.kubernetes.io/managed-by"],
			).To(Equal("epinio"))
		})
	})

	Describe("Update", func() {
		It("merges only non-empty fields into the spec", func() {
			existing := unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{"name": "standard"},
					"spec": map[string]interface{}{
						"image":            "old",
						"description":      "long",
						"shortDescription": "short",
					},
				},
			}
			fakeRI.GetReturns(&existing, nil)
			fakeRI.UpdateReturns(&unstructured.Unstructured{}, nil)

			req := models.BuilderImageUpdateRequest{
				Image: "new",
			}
			updateError := builderimage.Update(ctx, fakeNS, "standard", req)
			Expect(updateError).ToNot(HaveOccurred())

			_, sent, _, _ := fakeRI.UpdateArgsForCall(0)
			spec, _, _ := unstructured.NestedMap(sent.Object, "spec")
			Expect(spec["image"]).To(Equal("new"))
			Expect(spec["description"]).To(Equal("long"))
			Expect(spec["shortDescription"]).To(Equal("short"))
		})
	})

	Describe("Delete", func() {
		It("issues a delete to the dynamic client", func() {
			fakeRI.DeleteReturns(nil)
			deleteError := builderimage.Delete(ctx, fakeNS, "standard")
			Expect(deleteError).ToNot(HaveOccurred())
			Expect(fakeRI.DeleteCallCount()).To(Equal(1))

			_, name, _, _ := fakeRI.DeleteArgsForCall(0)
			Expect(name).To(Equal("standard"))
		})
	})
})
