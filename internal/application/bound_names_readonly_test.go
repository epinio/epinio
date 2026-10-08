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

// White-box test (package application) so a fake typed clientset can be injected
// on the cluster. The point of these specs is the read-only guarantee: the
// ...IfAny readers must never create the binding secret the way their
// loadOrCreateSecret-based counterparts do.
package application

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

var _ = Describe("read-only bound name readers", func() {
	var (
		ctx    context.Context
		appRef models.AppRef
	)

	BeforeEach(func() {
		ctx = context.Background()
		appRef = models.NewAppRef("myapp", "workspace")
	})

	clusterWith := func(objs ...runtime.Object) *kubernetes.Cluster {
		return &kubernetes.Cluster{Kubectl: k8sfake.NewSimpleClientset(objs...)}
	}

	bindingSecret := func(name string, bound ...string) *corev1.Secret {
		data := map[string][]byte{}
		for _, b := range bound {
			data[b] = nil
		}

		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "workspace",
			},
			Data: data,
		}
	}

	secretCount := func(cluster *kubernetes.Cluster) int {
		secrets, err := cluster.Kubectl.CoreV1().
			Secrets("workspace").List(ctx, metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())

		return len(secrets.Items)
	}

	Describe("BoundServiceNamesIfAny", func() {
		It("returns the bound service names, sorted", func() {
			cluster := clusterWith(
				bindingSecret(appRef.MakeServiceSecretName(), "redis", "mysql"),
			)

			Expect(BoundServiceNamesIfAny(ctx, cluster, appRef)).
				To(Equal([]string{"mysql", "redis"}))
		})

		It("reads a missing binding secret as no services, without creating it", func() {
			cluster := clusterWith()

			Expect(BoundServiceNamesIfAny(ctx, cluster, appRef)).To(BeEmpty())
			Expect(secretCount(cluster)).To(Equal(0))
		})
	})

	Describe("BoundConfigurationNamesIfAny", func() {
		It("returns the bound configuration names, sorted", func() {
			cluster := clusterWith(
				bindingSecret(appRef.MakeConfigurationSecretName(), "mine", "db-creds"),
			)

			Expect(BoundConfigurationNamesIfAny(ctx, cluster, appRef)).
				To(Equal([]string{"db-creds", "mine"}))
		})

		It("reads a missing binding secret as no configurations, without creating it", func() {
			cluster := clusterWith()

			Expect(BoundConfigurationNamesIfAny(ctx, cluster, appRef)).To(BeEmpty())
			Expect(secretCount(cluster)).To(Equal(0))
		})
	})
})
