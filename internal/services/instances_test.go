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
	"fmt"
	"math/rand"
	"time"

	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/internal/services/servicesfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
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
