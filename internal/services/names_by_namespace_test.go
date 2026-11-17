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

// White-box test (package services) so a ServiceClient can be built with a fake
// typed clientset. NamesByNamespace / NamesInNamespace only list labeled secrets.
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

var _ = Describe("NamesByNamespace", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

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

	It("groups service instance names by namespace", func() {
		client := clientWith(
			instanceSecret("r1", "ns1", "myredis", "redis-dev"),
			instanceSecret("p1", "ns2", "mypg", "postgresql-dev"),
			instanceSecret("r2", "ns1", "mycache", "redis-dev"),
		)

		byNS, err := client.NamesByNamespace(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(byNS).To(HaveLen(2))
		Expect(byNS["ns1"]).To(ConsistOf("myredis", "mycache"))
		Expect(byNS["ns2"]).To(ConsistOf("mypg"))
	})

	It("returns an empty map when there are no instances", func() {
		client := clientWith()

		byNS, err := client.NamesByNamespace(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(byNS).To(BeEmpty())
	})

	It("skips secrets without a service-name label", func() {
		noName := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "x",
				Namespace: "ns1",
				Labels: map[string]string{
					CatalogServiceLabelKey: "redis-dev",
				},
			},
		}
		client := clientWith(
			noName,
			instanceSecret("r1", "ns1", "myredis", "redis-dev"),
		)

		byNS, err := client.NamesByNamespace(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(byNS).To(HaveLen(1))
		Expect(byNS["ns1"]).To(Equal([]string{"myredis"}))
	})

	It("NamesInNamespace returns only that namespace's names", func() {
		client := clientWith(
			instanceSecret("r1", "ns1", "myredis", "redis-dev"),
			instanceSecret("p1", "ns2", "mypg", "postgresql-dev"),
		)

		names, err := client.NamesInNamespace(ctx, "ns2")
		Expect(err).ToNot(HaveOccurred())
		Expect(names).To(Equal([]string{"mypg"}))

		empty, err := client.NamesInNamespace(ctx, "missing")
		Expect(err).ToNot(HaveOccurred())
		Expect(empty).To(Equal([]string{}))
	})
})
