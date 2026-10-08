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

// White-box test (package services) so a ServiceClient can be built with only a
// fake typed clientset. serviceKubeClient stays nil on purpose: these specs
// assert that the name narrowing happens *before* the catalog lookup and the
// per-instance status fan-out, so a nil dynamic client is never reached.
package services

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("ListInNamespaceByNames", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	instanceSecret := func(serviceName, namespace string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceResourceName(serviceName),
				Namespace: namespace,
				Labels: map[string]string{
					ServiceNameLabelKey:    serviceName,
					CatalogServiceLabelKey: "mysql-dev",
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

	It("returns an empty list for an empty name list, without touching the cluster", func() {
		client := &ServiceClient{}

		Expect(client.ListInNamespaceByNames(ctx, "workspace", nil)).
			To(Equal(models.ServiceList{}))
		Expect(client.ListInNamespaceByNames(ctx, "workspace", []string{})).
			To(Equal(models.ServiceList{}))
	})

	It("drops the instances not named, before the catalog and status lookups", func() {
		client := clientWith(
			instanceSecret("mysql", "workspace"),
			instanceSecret("redis", "workspace"),
		)

		// A nil serviceKubeClient would panic in ListCatalogServices. Reaching the
		// end means every instance in the namespace was filtered out first.
		Expect(client.ListInNamespaceByNames(ctx, "workspace", []string{"postgres"})).
			To(Equal(models.ServiceList{}))
	})

	It("ignores instances of the same name in another namespace", func() {
		client := clientWith(instanceSecret("mysql", "other"))

		Expect(client.ListInNamespaceByNames(ctx, "workspace", []string{"mysql"})).
			To(Equal(models.ServiceList{}))
	})
})
