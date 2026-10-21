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

// White-box test (package services) for ListRefs. Only a fake typed clientset
// is wired: no dynamic client and no helm, so any status lookup would fail.
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

var _ = Describe("ListRefs", func() {
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

	clusterWith := func(objs ...runtime.Object) *kubernetes.Cluster {
		return &kubernetes.Cluster{
			Kubectl: k8sfake.NewSimpleClientset(objs...),
		}
	}

	names := func(refs []models.Meta) []string {
		out := make([]string, 0, len(refs))
		for _, ref := range refs {
			out = append(out, ref.Namespace+"/"+ref.Name)
		}
		return out
	}

	It("returns an empty list when there are no instances", func() {
		Expect(ListRefs(ctx, clusterWith(), "")).To(BeEmpty())
	})

	It("uses the service name label, not the secret name", func() {
		refs, err := ListRefs(
			ctx,
			clusterWith(instanceSecret("mysql", "workspace")),
			"",
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(names(refs)).To(Equal([]string{"workspace/mysql"}))
	})

	It("lists across all namespaces when the namespace is blank", func() {
		refs, err := ListRefs(ctx, clusterWith(
			instanceSecret("mysql", "workspace"),
			instanceSecret("redis", "other"),
		), "")
		Expect(err).ToNot(HaveOccurred())
		Expect(names(refs)).To(ConsistOf("workspace/mysql", "other/redis"))
	})

	It("restricts to the given namespace", func() {
		refs, err := ListRefs(ctx, clusterWith(
			instanceSecret("mysql", "workspace"),
			instanceSecret("redis", "other"),
		), "other")
		Expect(err).ToNot(HaveOccurred())
		Expect(names(refs)).To(Equal([]string{"other/redis"}))
	})

	It("skips secrets without both instance labels", func() {
		plain := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "registry-creds",
				Namespace: "workspace",
				Labels:    map[string]string{ServiceNameLabelKey: "x"},
			},
		}
		refs, err := ListRefs(ctx, clusterWith(
			plain,
			instanceSecret("mysql", "workspace"),
		), "")
		Expect(err).ToNot(HaveOccurred())
		Expect(names(refs)).To(Equal([]string{"workspace/mysql"}))
	})
})
