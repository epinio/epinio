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

// White-box test (package services) so we can construct a ServiceClient
// with an injected fake dynamic client. The methods under test only use
// serviceKubeClient; kubeClient stays nil and we avoid paths that would
// dereference it.
package services

import (
	"context"

	"github.com/epinio/epinio/internal/testfakes/k8sdynamic/k8sdynamicfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Catalog Service CRUD", func() {
	var (
		ctx     context.Context
		fakeRI  *k8sdynamicfakes.FakeResourceInterface
		fakeNS  *k8sdynamicfakes.FakeNamespaceableResourceInterface
		client  *ServiceClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeRI = &k8sdynamicfakes.FakeResourceInterface{}
		fakeNS = &k8sdynamicfakes.FakeNamespaceableResourceInterface{}
		fakeNS.NamespaceReturns(fakeRI)

		client = &ServiceClient{
			serviceKubeClient: fakeNS,
		}
	})

	Describe("CatalogServiceExists", func() {
		It("returns true when the resource is present", func() {
			fakeRI.GetReturns(&unstructured.Unstructured{}, nil)

			exists, existsError := client.CatalogServiceExists(ctx, "redis-dev")

			Expect(existsError).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
			Expect(fakeRI.GetCallCount()).To(Equal(1))

			_, name, _, _ := fakeRI.GetArgsForCall(0)
			Expect(name).To(Equal("redis-dev"))
		})

		It("returns false when the resource is not found", func() {
			fakeRI.GetReturns(
				nil,
				k8sapierrors.NewNotFound(
					schema.GroupResource{Resource: "services"},
					"redis-dev",
				),
			)

			exists, existsError := client.CatalogServiceExists(ctx, "redis-dev")

			Expect(existsError).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("propagates non-NotFound errors", func() {
			fakeRI.GetReturns(
				nil,
				k8sapierrors.NewInternalError(
					assertGenericError("boom"),
				),
			)

			exists, existsError := client.CatalogServiceExists(ctx, "redis-dev")

			Expect(existsError).To(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})

	Describe("CreateCatalogService", func() {
		It("creates the resource with the supplied spec", func() {
			fakeRI.CreateReturns(&unstructured.Unstructured{}, nil)

			req := models.CatalogServiceCreateRequest{
				Name:             "postgresql-test",
				ShortDescription: "test",
				Description:      "long",
				HelmChart:        "postgresql",
				ChartVersion:     "12.1.6",
				HelmRepo: models.HelmRepoRequest{
					Name: "bitnami",
					URL:  "https://charts.bitnami.com/bitnami",
				},
				SecretTypes: []string{"Opaque"},
			}

			_, createError := client.CreateCatalogService(ctx, req)

			Expect(createError).ToNot(HaveOccurred())
			Expect(fakeRI.CreateCallCount()).To(Equal(1))

			_, sent, _, _ := fakeRI.CreateArgsForCall(0)
			Expect(sent.GetKind()).To(Equal("Service"))
			Expect(sent.GetAPIVersion()).To(Equal("application.epinio.io/v1"))
			Expect(sent.GetName()).To(Equal("postgresql-test"))
			Expect(
				sent.GetAnnotations()[CatalogServiceSecretTypesAnnotation],
			).To(Equal("Opaque"))

			spec, _, specError := unstructured.NestedMap(
				sent.Object,
				"spec",
			)
			Expect(specError).ToNot(HaveOccurred())
			Expect(spec["chart"]).To(Equal("postgresql"))
			Expect(spec["chartVersion"]).To(Equal("12.1.6"))
		})

		It("returns the underlying error on failure", func() {
			fakeRI.CreateReturns(
				nil,
				k8sapierrors.NewInternalError(assertGenericError("boom")),
			)

			_, createError := client.CreateCatalogService(
				ctx,
				models.CatalogServiceCreateRequest{Name: "x", HelmChart: "c"},
			)

			Expect(createError).To(HaveOccurred())
		})
	})

	Describe("UpdateCatalogService", func() {
		It("patches only the supplied fields", func() {
			existing := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "redis-dev",
					},
					"spec": map[string]interface{}{
						"description": "old description",
						"chart":       "redis",
					},
				},
			}
			fakeRI.GetReturns(existing, nil)
			fakeRI.UpdateReturns(&unstructured.Unstructured{}, nil)

			req := models.CatalogServiceUpdateRequest{
				ShortDescription: "new short",
			}

			updateError := client.UpdateCatalogService(ctx, "redis-dev", req)

			Expect(updateError).ToNot(HaveOccurred())
			Expect(fakeRI.UpdateCallCount()).To(Equal(1))

			_, sent, _, _ := fakeRI.UpdateArgsForCall(0)
			spec, _, _ := unstructured.NestedMap(sent.Object, "spec")
			Expect(spec["shortDescription"]).To(Equal("new short"))
			Expect(spec["description"]).To(Equal("old description"))
			Expect(spec["chart"]).To(Equal("redis"))
		})

		It("clears SecretTypes annotation when given an empty slice", func() {
			existing := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "redis-dev",
						"annotations": map[string]interface{}{
							CatalogServiceSecretTypesAnnotation: "Opaque",
						},
					},
					"spec": map[string]interface{}{},
				},
			}
			fakeRI.GetReturns(existing, nil)
			fakeRI.UpdateReturns(&unstructured.Unstructured{}, nil)

			req := models.CatalogServiceUpdateRequest{
				SecretTypes: []string{},
			}

			updateError := client.UpdateCatalogService(ctx, "redis-dev", req)
			Expect(updateError).ToNot(HaveOccurred())

			_, sent, _, _ := fakeRI.UpdateArgsForCall(0)
			Expect(
				sent.GetAnnotations(),
			).ToNot(HaveKey(CatalogServiceSecretTypesAnnotation))
		})

		It("returns the underlying error when the Get fails", func() {
			fakeRI.GetReturns(
				nil,
				k8sapierrors.NewInternalError(assertGenericError("boom")),
			)

			updateError := client.UpdateCatalogService(
				ctx,
				"redis-dev",
				models.CatalogServiceUpdateRequest{},
			)
			Expect(updateError).To(HaveOccurred())
		})
	})

	Describe("DeleteCatalogService", func() {
		It("issues a delete to the dynamic client", func() {
			fakeRI.DeleteReturns(nil)

			deleteError := client.DeleteCatalogService(ctx, "redis-dev")

			Expect(deleteError).ToNot(HaveOccurred())
			Expect(fakeRI.DeleteCallCount()).To(Equal(1))

			_, name, _, _ := fakeRI.DeleteArgsForCall(0)
			Expect(name).To(Equal("redis-dev"))
		})

		It("propagates errors", func() {
			fakeRI.DeleteReturns(
				k8sapierrors.NewInternalError(assertGenericError("boom")),
			)

			deleteError := client.DeleteCatalogService(ctx, "redis-dev")
			Expect(deleteError).To(HaveOccurred())
		})
	})
})

// assertGenericError builds a minimal error usable as the cause for
// k8sapierrors.NewInternalError without pulling in extra deps.
func assertGenericError(msg string) error {
	return &genericError{msg: msg}
}

type genericError struct{ msg string }

func (g *genericError) Error() string { return g.msg }

// staticListOptions is unused — silences an unused-import nag if metav1
// gets dropped during refactors.
var _ = metav1.ListOptions{}
