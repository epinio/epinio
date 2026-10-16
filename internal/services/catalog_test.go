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

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/testfakes/k8sdynamic/k8sdynamicfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var _ = Describe("Catalog Service CRUD", func() {
	var (
		ctx    context.Context
		fakeRI *k8sdynamicfakes.FakeResourceInterface
		fakeNS *k8sdynamicfakes.FakeNamespaceableResourceInterface
		client *ServiceClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		helpers.Logger = zap.NewNop().Sugar()
		viper.Set("namespace", "epinio")

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

	Describe("helm repo auth secret", func() {
		var fakeKube *k8sfake.Clientset

		catalogCR := func(name, secretName string) *unstructured.Unstructured {
			return &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": name,
					},
					"spec": map[string]interface{}{
						"chart": "redis",
						"helmRepo": map[string]interface{}{
							"name":   "repo",
							"url":    "https://charts.example.test",
							"secret": secretName,
						},
					},
				},
			}
		}

		BeforeEach(func() {
			fakeKube = k8sfake.NewSimpleClientset()
			client.kubeClient = &kubernetes.Cluster{Kubectl: fakeKube}
		})

		It("loads the credentials when the secret is present", func() {
			fakeKube = k8sfake.NewSimpleClientset(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "repo-creds",
					Namespace: "epinio",
				},
				Data: map[string][]byte{
					"username": []byte("repo-user"),
					"password": []byte("repo-pass"),
				},
			})
			client.kubeClient = &kubernetes.Cluster{Kubectl: fakeKube}
			fakeRI.GetReturns(catalogCR("redis-dev", "repo-creds"), nil)

			service, getError := client.GetCatalogService(ctx, "redis-dev")

			Expect(getError).ToNot(HaveOccurred())
			Expect(service.HelmRepo.Auth.Username).To(Equal("repo-user"))
			Expect(service.HelmRepo.Auth.Password).To(Equal("repo-pass"))
		})

		It("returns the entry without credentials when the secret is gone", func() {
			fakeRI.GetReturns(catalogCR("redis-dev", "gone"), nil)

			service, getError := client.GetCatalogService(ctx, "redis-dev")

			Expect(getError).ToNot(HaveOccurred())
			Expect(service.Meta.Name).To(Equal("redis-dev"))
			Expect(service.HelmRepo.URL).To(Equal("https://charts.example.test"))
			Expect(service.HelmRepo.Auth.Username).To(BeEmpty())
			Expect(service.HelmRepo.Auth.Password).To(BeEmpty())
		})

		It("keeps listing the other entries when one secret is gone", func() {
			fakeRI.ListReturns(&unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{
					*catalogCR("redis-dev", ""),
					*catalogCR("mysql-dev", "gone"),
				},
			}, nil)

			services, listError := client.ListCatalogServices(ctx)

			Expect(listError).ToNot(HaveOccurred())
			Expect(services).To(HaveLen(2))
			Expect(services[1].Meta.Name).To(Equal("mysql-dev"))
			Expect(services[1].HelmRepo.Auth.Username).To(BeEmpty())
		})

		It("yields empty credentials when the secret has no keys", func() {
			fakeKube = k8sfake.NewSimpleClientset(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "repo-creds",
					Namespace: "epinio",
				},
			})
			client.kubeClient = &kubernetes.Cluster{Kubectl: fakeKube}
			fakeRI.GetReturns(catalogCR("redis-dev", "repo-creds"), nil)

			service, getError := client.GetCatalogService(ctx, "redis-dev")

			Expect(getError).ToNot(HaveOccurred())
			Expect(service.HelmRepo.Auth.Username).To(BeEmpty())
			Expect(service.HelmRepo.Auth.Password).To(BeEmpty())
		})

		It("propagates errors that are not NotFound", func() {
			fakeKube.PrependReactor(
				"get",
				"secrets",
				func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, k8sapierrors.NewInternalError(
						assertGenericError("boom"),
					)
				},
			)
			fakeRI.GetReturns(catalogCR("redis-dev", "repo-creds"), nil)

			_, getError := client.GetCatalogService(ctx, "redis-dev")

			Expect(getError).To(HaveOccurred())
			Expect(
				getError.Error(),
			).To(ContainSubstring("finding helm repo auth secret: repo-creds"))
		})
	})

	// The create and update paths guard the secret lookup so that a catalog
	// service which names no secret never triggers one. These specs pin both
	// gates, the lookup itself, and what happens when it fails.
	Describe("helm repo secret validation on write", func() {
		var (
			fakeKube   *k8sfake.Clientset
			secretGets []string
			secretNSs  []string
		)

		// newKube rebuilds the fake cluster, seeding it with the given
		// objects and recording every secret Get that reaches it.
		newKube := func(objects ...runtime.Object) {
			fakeKube = k8sfake.NewSimpleClientset(objects...)
			secretGets = nil
			secretNSs = nil

			fakeKube.PrependReactor(
				"get",
				"secrets",
				func(action k8stesting.Action) (bool, runtime.Object, error) {
					getAction := action.(k8stesting.GetAction)
					secretGets = append(secretGets, getAction.GetName())
					secretNSs = append(secretNSs, getAction.GetNamespace())
					// Not handled here; fall through to the tracker.
					return false, nil, nil
				},
			)

			client.kubeClient = &kubernetes.Cluster{Kubectl: fakeKube}
		}

		repoSecret := func(name string) *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "epinio",
				},
				Data: map[string][]byte{
					"username": []byte("repo-user"),
					"password": []byte("repo-pass"),
				},
			}
		}

		existingCR := func() *unstructured.Unstructured {
			return &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "redis-dev",
					},
					"spec": map[string]interface{}{
						"chart": "redis",
					},
				},
			}
		}

		BeforeEach(func() {
			newKube()
		})

		Describe("CreateCatalogService", func() {
			It("skips the lookup when no secret is referenced", func() {
				fakeRI.CreateReturns(&unstructured.Unstructured{}, nil)

				req := models.CatalogServiceCreateRequest{
					Name:      "postgresql-test",
					HelmChart: "postgresql",
					HelmRepo: models.HelmRepoRequest{
						Name: "bitnami",
						URL:  "https://charts.bitnami.com/bitnami",
					},
				}

				_, createError := client.CreateCatalogService(ctx, req)

				Expect(createError).ToNot(HaveOccurred())
				Expect(secretGets).To(BeEmpty())
				Expect(fakeRI.CreateCallCount()).To(Equal(1))
			})

			It("looks the secret up in the epinio namespace", func() {
				newKube(repoSecret("repo-creds"))
				fakeRI.CreateReturns(&unstructured.Unstructured{}, nil)

				req := models.CatalogServiceCreateRequest{
					Name:      "postgresql-test",
					HelmChart: "postgresql",
					HelmRepo: models.HelmRepoRequest{
						Name:   "private",
						URL:    "https://charts.example.test",
						Secret: "repo-creds",
					},
				}

				_, createError := client.CreateCatalogService(ctx, req)

				Expect(createError).ToNot(HaveOccurred())
				Expect(secretGets).To(Equal([]string{"repo-creds"}))
				Expect(secretNSs).To(Equal([]string{"epinio"}))

				_, sent, _, _ := fakeRI.CreateArgsForCall(0)
				repo, _, repoError := unstructured.NestedMap(
					sent.Object,
					"spec",
					"helmRepo",
				)
				Expect(repoError).ToNot(HaveOccurred())
				Expect(repo["secret"]).To(Equal("repo-creds"))
			})

			It("rejects the request and creates nothing when the "+
				"secret is missing", func() {
				req := models.CatalogServiceCreateRequest{
					Name:      "postgresql-test",
					HelmChart: "postgresql",
					HelmRepo: models.HelmRepoRequest{
						Secret: "gone",
					},
				}

				_, createError := client.CreateCatalogService(ctx, req)

				Expect(createError).To(HaveOccurred())
				Expect(createError.Error()).To(ContainSubstring(
					"Something went wrong getting helm repo secret gone",
				))
				Expect(
					k8sapierrors.IsNotFound(errors.Cause(createError)),
				).To(BeTrue())
				Expect(fakeRI.CreateCallCount()).To(Equal(0))
			})

			It("rejects the request when the lookup fails for another "+
				"reason", func() {
				fakeKube.PrependReactor(
					"get",
					"secrets",
					func(k8stesting.Action) (bool, runtime.Object, error) {
						return true, nil, k8sapierrors.NewInternalError(
							assertGenericError("boom"),
						)
					},
				)

				req := models.CatalogServiceCreateRequest{
					Name:      "postgresql-test",
					HelmChart: "postgresql",
					HelmRepo: models.HelmRepoRequest{
						Secret: "repo-creds",
					},
				}

				_, createError := client.CreateCatalogService(ctx, req)

				Expect(createError).To(HaveOccurred())
				Expect(
					k8sapierrors.IsNotFound(errors.Cause(createError)),
				).To(BeFalse())
				Expect(fakeRI.CreateCallCount()).To(Equal(0))
			})
		})

		Describe("UpdateCatalogService", func() {
			It("skips the lookup when the request omits helm_repo", func() {
				fakeRI.GetReturns(existingCR(), nil)
				fakeRI.UpdateReturns(&unstructured.Unstructured{}, nil)

				updateError := client.UpdateCatalogService(
					ctx,
					"redis-dev",
					models.CatalogServiceUpdateRequest{
						ShortDescription: "new short",
					},
				)

				Expect(updateError).ToNot(HaveOccurred())
				Expect(secretGets).To(BeEmpty())
				Expect(fakeRI.UpdateCallCount()).To(Equal(1))
			})

			It("skips the lookup when helm_repo carries no secret", func() {
				fakeRI.GetReturns(existingCR(), nil)
				fakeRI.UpdateReturns(&unstructured.Unstructured{}, nil)

				updateError := client.UpdateCatalogService(
					ctx,
					"redis-dev",
					models.CatalogServiceUpdateRequest{
						HelmRepo: &models.HelmRepoRequest{
							Name: "public",
							URL:  "https://charts.example.test",
						},
					},
				)

				Expect(updateError).ToNot(HaveOccurred())
				Expect(secretGets).To(BeEmpty())
				Expect(fakeRI.UpdateCallCount()).To(Equal(1))
			})

			It("looks the secret up and patches it into the spec", func() {
				newKube(repoSecret("repo-creds"))
				fakeRI.GetReturns(existingCR(), nil)
				fakeRI.UpdateReturns(&unstructured.Unstructured{}, nil)

				updateError := client.UpdateCatalogService(
					ctx,
					"redis-dev",
					models.CatalogServiceUpdateRequest{
						HelmRepo: &models.HelmRepoRequest{
							Name:   "private",
							URL:    "https://charts.example.test",
							Secret: "repo-creds",
						},
					},
				)

				Expect(updateError).ToNot(HaveOccurred())
				Expect(secretGets).To(Equal([]string{"repo-creds"}))
				Expect(secretNSs).To(Equal([]string{"epinio"}))

				_, sent, _, _ := fakeRI.UpdateArgsForCall(0)
				repo, _, repoError := unstructured.NestedMap(
					sent.Object,
					"spec",
					"helmRepo",
				)
				Expect(repoError).ToNot(HaveOccurred())
				Expect(repo["secret"]).To(Equal("repo-creds"))
			})

			It("rejects the update and touches nothing when the "+
				"secret is missing", func() {
				fakeRI.GetReturns(existingCR(), nil)

				updateError := client.UpdateCatalogService(
					ctx,
					"redis-dev",
					models.CatalogServiceUpdateRequest{
						HelmRepo: &models.HelmRepoRequest{
							Secret: "gone",
						},
					},
				)

				Expect(updateError).To(HaveOccurred())
				Expect(updateError.Error()).To(ContainSubstring(
					"Something went wrong getting helm repo secret gone",
				))
				Expect(
					k8sapierrors.IsNotFound(errors.Cause(updateError)),
				).To(BeTrue())
				// The gate runs before the CR is fetched, so a bad secret
				// leaves the existing catalog service untouched.
				Expect(fakeRI.GetCallCount()).To(Equal(0))
				Expect(fakeRI.UpdateCallCount()).To(Equal(0))
			})
		})

		// The write guard only holds at write time. Nothing stops an
		// operator deleting the secret afterwards, which is exactly how
		// the catalog listing broke in the first place, so the read path
		// has to survive it.
		It("still lists the entry after the secret is deleted", func() {
			newKube(repoSecret("repo-creds"))
			fakeRI.CreateReturns(&unstructured.Unstructured{}, nil)

			req := models.CatalogServiceCreateRequest{
				Name:      "postgresql-test",
				HelmChart: "postgresql",
				HelmRepo: models.HelmRepoRequest{
					Name:   "private",
					URL:    "https://charts.example.test",
					Secret: "repo-creds",
				},
			}

			_, createError := client.CreateCatalogService(ctx, req)
			Expect(createError).ToNot(HaveOccurred())

			// Feed the CR that Create actually built back to the reader,
			// so the round-trip is the real one and not a hand-written CR.
			_, created, _, _ := fakeRI.CreateArgsForCall(0)

			deleteError := fakeKube.CoreV1().Secrets("epinio").Delete(
				ctx,
				"repo-creds",
				metav1.DeleteOptions{},
			)
			Expect(deleteError).ToNot(HaveOccurred())

			fakeRI.ListReturns(&unstructured.UnstructuredList{
				Items: []unstructured.Unstructured{*created},
			}, nil)
			fakeRI.GetReturns(created, nil)

			services, listError := client.ListCatalogServices(ctx)

			Expect(listError).ToNot(HaveOccurred())
			Expect(services).To(HaveLen(1))
			Expect(services[0].Meta.Name).To(Equal("postgresql-test"))
			Expect(services[0].HelmRepo.URL).To(
				Equal("https://charts.example.test"),
			)
			Expect(services[0].HelmRepo.Auth.Username).To(BeEmpty())

			service, getError := client.GetCatalogService(
				ctx,
				"postgresql-test",
			)

			Expect(getError).ToNot(HaveOccurred())
			Expect(service.HelmRepo.Auth.Username).To(BeEmpty())
			Expect(service.HelmRepo.Auth.Password).To(BeEmpty())
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
