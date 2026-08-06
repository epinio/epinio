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

// White-box test (package services) so we can construct a ServiceClient with a
// fake typed clientset injected on kubeClient. CatalogServicesInUse only uses
// kubeClient.Kubectl; serviceKubeClient stays nil.
package services

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("CatalogServicesInUse", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	// instanceSecret builds a service-instance marker secret the way Create
	// labels it: both the service-name and catalog-service-name labels present.
	instanceSecret := func(name, namespace, serviceName, catalogService string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					ServiceNameLabelKey:    serviceName,
					CatalogServiceLabelKey: catalogService,
				},
			},
		}
	}

	clientWith := func(objs ...runtime.Object) *ServiceClient {
		return &ServiceClient{
			kubeClient: &kubernetes.Cluster{
				Kubectl: k8sfake.NewSimpleClientset(objs...),
			},
		}
	}

	It("returns the set of catalog services with at least one instance, across namespaces", func() {
		client := clientWith(
			instanceSecret("r1", "ns1", "myredis", "redis-dev"),
			instanceSecret("p1", "ns2", "mypg", "postgresql-dev"),
			// second instance of the same catalog service collapses to one key
			instanceSecret("r2", "ns2", "myredis2", "redis-dev"),
		)

		inUse, err := client.CatalogServicesInUse(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(inUse).To(HaveLen(2))
		Expect(inUse).To(HaveKeyWithValue("redis-dev", true))
		Expect(inUse).To(HaveKeyWithValue("postgresql-dev", true))
	})

	It("returns an empty set when there are no instances", func() {
		client := clientWith()

		inUse, err := client.CatalogServicesInUse(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(inUse).To(BeEmpty())
	})

	It("skips secrets without a catalog-service-name label", func() {
		noLabel := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns1"},
		}
		client := clientWith(
			noLabel,
			instanceSecret("r1", "ns1", "myredis", "redis-dev"),
		)

		inUse, err := client.CatalogServicesInUse(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(inUse).To(HaveLen(1))
		Expect(inUse).To(HaveKey("redis-dev"))
	})
})
