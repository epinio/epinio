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

package services_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/internal/services/servicesfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	k8stesting "k8s.io/client-go/testing"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate -header ../../LICENSE_HEADER k8s.io/client-go/kubernetes/typed/core/v1.ServiceInterface
type MockServiceInterface struct {
	v1.ServiceInterface
}

var r *rand.Rand

var _ = Describe("Service Instances", func() {

	var fake *servicesfakes.FakeServiceInterface
	var ctx context.Context
	var name, namespace string

	BeforeEach(func() {
		ctx = context.Background()
		r = rand.New(rand.NewSource(time.Now().UnixNano()))
		fake = &servicesfakes.FakeServiceInterface{}

		name = "service-name"
		namespace = "service-namespace"
	})

	Describe("GetInternalRoutes", func() {

		When("a service with one port 80 is returned", func() {
			It("returns one route with no port", func() {
				fake.ListReturns(
					newServiceList(newService(name, namespace, []int32{80})),
					nil,
				)

				expectedRoutes := []string{fmt.Sprintf("%s.%s.svc.cluster.local", name, namespace)}

				internalRoutes, err := services.GetInternalRoutes(ctx, fake, name)
				Expect(err).To(BeNil())
				Expect(internalRoutes).To(Not(BeNil()))
				Expect(internalRoutes).To(HaveLen(len(expectedRoutes)))
				Expect(internalRoutes).To(BeEquivalentTo(expectedRoutes))

				_, listOpts := fake.ListArgsForCall(0)
				Expect(listOpts).To(Not(BeNil()))
				Expect(listOpts).To(BeEquivalentTo(metav1.ListOptions{
					LabelSelector: "app.kubernetes.io/instance=" + names.ServiceReleaseName(name),
				}))
			})
		})

		When("a service with two ports is returned", func() {
			It("returns two routes with the corresponding ports", func() {
				fake.ListReturns(
					newServiceList(newService(name, namespace, []int32{80, 443})),
					nil,
				)

				expectedRoutes := []string{
					fmt.Sprintf("%s.%s.svc.cluster.local", name, namespace),
					fmt.Sprintf("%s.%s.svc.cluster.local:%d", name, namespace, 443),
				}

				internalRoutes, err := services.GetInternalRoutes(ctx, fake, name)
				Expect(err).To(BeNil())
				Expect(internalRoutes).To(Not(BeNil()))
				Expect(internalRoutes).To(HaveLen(len(expectedRoutes)))
				Expect(internalRoutes).To(BeEquivalentTo(expectedRoutes))

				_, listOpts := fake.ListArgsForCall(0)
				Expect(listOpts).To(Not(BeNil()))
				Expect(listOpts).To(BeEquivalentTo(metav1.ListOptions{
					LabelSelector: "app.kubernetes.io/instance=" + names.ServiceReleaseName(name),
				}))
			})
		})

		When("multiple services with different ports are returned", func() {
			It("returns multiple routes with the corresponding ports", func() {
				fake.ListReturns(
					newServiceList(
						newService(name, namespace, []int32{80, 443}),
						newService(name+"-master", namespace, []int32{3306}),
						newService(name+"-replica", namespace, []int32{22700, 5005}),
					),
					nil,
				)

				expectedRoutes := []string{
					fmt.Sprintf("%s.%s.svc.cluster.local", name, namespace),
					fmt.Sprintf("%s.%s.svc.cluster.local:%d", name, namespace, 443),
					fmt.Sprintf("%s-master.%s.svc.cluster.local:%d", name, namespace, 3306),
					fmt.Sprintf("%s-replica.%s.svc.cluster.local:%d", name, namespace, 22700),
					fmt.Sprintf("%s-replica.%s.svc.cluster.local:%d", name, namespace, 5005),
				}

				internalRoutes, err := services.GetInternalRoutes(ctx, fake, name)
				Expect(err).To(BeNil())
				Expect(internalRoutes).To(Not(BeNil()))
				Expect(internalRoutes).To(HaveLen(len(expectedRoutes)))
				Expect(internalRoutes).To(BeEquivalentTo(expectedRoutes))

				_, listOpts := fake.ListArgsForCall(0)
				Expect(listOpts).To(Not(BeNil()))
				Expect(listOpts).To(BeEquivalentTo(metav1.ListOptions{
					LabelSelector: "app.kubernetes.io/instance=" + names.ServiceReleaseName(name),
				}))
			})
		})

		When("no services are returned", func() {
			It("returns no routes", func() {
				fake.ListReturns(newServiceList(), nil)

				internalRoutes, err := services.GetInternalRoutes(ctx, fake, name)
				Expect(err).To(BeNil())
				Expect(internalRoutes).To(Not(BeNil()))
				Expect(internalRoutes).To(BeEmpty())

				_, listOpts := fake.ListArgsForCall(0)
				Expect(listOpts).To(Not(BeNil()))
				Expect(listOpts).To(BeEquivalentTo(metav1.ListOptions{
					LabelSelector: "app.kubernetes.io/instance=" + names.ServiceReleaseName(name),
				}))
			})
		})

		When("an error occurred", func() {
			It("returns an error and no routes", func() {
				fake.ListReturns(nil, fmt.Errorf("something bad happened"))

				internalRoutes, err := services.GetInternalRoutes(ctx, fake, name)
				Expect(err).To(Not(BeNil()))
				Expect(internalRoutes).To(BeNil())

				_, listOpts := fake.ListArgsForCall(0)
				Expect(listOpts).To(Not(BeNil()))
				Expect(listOpts).To(BeEquivalentTo(metav1.ListOptions{
					LabelSelector: "app.kubernetes.io/instance=" + names.ServiceReleaseName(name),
				}))
			})
		})
	})

	Describe("UpdateSettings", func() {
		const namespace = "workspace"
		const secretName = "s-myservice"

		var client *k8sfake.Clientset

		newSecret := func(settings models.ChartValueSettings) *corev1.Secret {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
			}
			if settings != nil {
				data, err := yaml.Marshal(settings)
				Expect(err).ToNot(HaveOccurred())
				secret.Data = map[string][]byte{"settings": data}
			}
			return secret
		}

		storedSettings := func() models.ChartValueSettings {
			secret, err := client.CoreV1().Secrets(namespace).
				Get(ctx, secretName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			settings := models.ChartValueSettings{}
			err = yaml.Unmarshal(secret.Data["settings"], &settings)
			Expect(err).ToNot(HaveOccurred())
			return settings
		}

		seed := func(settings models.ChartValueSettings) {
			client = k8sfake.NewSimpleClientset(newSecret(settings))
		}

		update := func(
			changes models.ServiceUpdateRequest,
		) (models.ChartValueSettings, error) {
			return services.UpdateSettings(
				ctx,
				client.CoreV1().Secrets(namespace),
				secretName,
				changes,
			)
		}

		// EPINIO-814: the returned settings feed the helm redeploy. They were
		// nil on success, so the release lost every setting.
		It("returns the untouched settings along with the changed one", func() {
			seed(models.ChartValueSettings{
				"s3.replicas":   "2",
				"s3.enableAuth": "true",
			})

			settings, err := update(models.ServiceUpdateRequest{
				Set: models.ChartValueSettings{"s3.replicas": "3"},
			})
			Expect(err).ToNot(HaveOccurred())

			expected := models.ChartValueSettings{
				"s3.replicas":   "3",
				"s3.enableAuth": "true",
			}
			Expect(settings).To(Equal(expected))
			Expect(storedSettings()).To(Equal(expected))
		})

		It("applies removals before additions", func() {
			seed(models.ChartValueSettings{
				"ingress.enabled":    "true",
				"ingress.hostname":   "example.com",
				"nesting.here.hello": "world",
			})

			settings, err := update(models.ServiceUpdateRequest{
				Remove: []string{"ingress.enabled", "nesting.here.hello"},
				Set:    models.ChartValueSettings{"nesting.here.hello": "user"},
			})
			Expect(err).ToNot(HaveOccurred())

			expected := models.ChartValueSettings{
				"ingress.hostname":   "example.com",
				"nesting.here.hello": "user",
			}
			Expect(settings).To(Equal(expected))
			Expect(storedSettings()).To(Equal(expected))
		})

		It("starts from empty settings when the secret has none", func() {
			seed(nil)

			settings, err := update(models.ServiceUpdateRequest{
				Set: models.ChartValueSettings{"a": "1"},
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(settings).To(Equal(models.ChartValueSettings{"a": "1"}))
			Expect(storedSettings()).To(
				Equal(models.ChartValueSettings{"a": "1"}))
		})

		It("retries on conflict and returns the final settings", func() {
			seed(models.ChartValueSettings{
				"a": "1",
			})

			conflicts := 1
			client.PrependReactor("update", "secrets",
				func(k8stesting.Action) (bool, runtime.Object, error) {
					if conflicts == 0 {
						return false, nil, nil
					}
					conflicts--
					return true, nil, apierrors.NewConflict(
						schema.GroupResource{Resource: "secrets"},
						secretName,
						errors.New("stale"),
					)
				})

			settings, err := update(models.ServiceUpdateRequest{
				Set: models.ChartValueSettings{"b": "2"},
			})
			Expect(err).ToNot(HaveOccurred())

			expected := models.ChartValueSettings{"a": "1", "b": "2"}
			Expect(settings).To(Equal(expected))
			Expect(storedSettings()).To(Equal(expected))
		})

		It("returns the error and no settings when the write fails", func() {
			seed(models.ChartValueSettings{
				"a": "1",
			})
			client.PrependReactor("update", "secrets",
				func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("boom")
				})

			settings, err := update(models.ServiceUpdateRequest{
				Set: models.ChartValueSettings{"b": "2"},
			})
			Expect(err).To(MatchError("boom"))
			Expect(settings).To(BeNil())
			Expect(storedSettings()).To(
				Equal(models.ChartValueSettings{"a": "1"}))
		})

		It("returns the error when the secret does not exist", func() {
			client = k8sfake.NewSimpleClientset()

			settings, err := update(models.ServiceUpdateRequest{
				Set: models.ChartValueSettings{"a": "1"},
			})
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "got %v", err)
			Expect(settings).To(BeNil())
		})
	})
})

func newServiceList(services ...corev1.Service) *corev1.ServiceList {
	return &corev1.ServiceList{Items: services}
}

func newService(name, namespace string, ports []int32) corev1.Service {
	servicePorts := []corev1.ServicePort{}
	for _, port := range ports {
		servicePorts = append(servicePorts, corev1.ServicePort{Port: port})
	}

	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{Ports: servicePorts},
	}
}
