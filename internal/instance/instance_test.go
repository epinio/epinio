// Copyright © 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package instance_test

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/instance"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("instance identity", func() {
	const ns = "epinio-test"

	var (
		ctx     context.Context
		cluster *kubernetes.Cluster
	)

	BeforeEach(func() {
		instance.ResetCache()
		ctx = context.Background()
		viper.Set("namespace", ns)
		viper.Set("install-method", "helm")

		cluster = &kubernetes.Cluster{
			Kubectl: fake.NewSimpleClientset(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: ns},
			}),
		}
	})

	AfterEach(func() {
		instance.ResetCache()
		viper.Set("namespace", "")
		viper.Set("install-method", "")
	})

	It("Get reports ErrNotFound when the ConfigMap is missing and does not create it", func() {
		_, err := instance.Get(ctx, cluster)
		Expect(err).To(MatchError(instance.ErrNotFound))

		_, err = cluster.Kubectl.CoreV1().ConfigMaps(ns).Get(ctx, helmchart.EpinioInstanceConfigMapName, metav1.GetOptions{})
		Expect(err).To(HaveOccurred())
		_, ok := instance.GetCached()
		Expect(ok).To(BeFalse())
	})

	It("Create makes a ConfigMap with a new identity", func() {
		info, err := instance.Create(ctx, cluster)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.ID).ToNot(BeEmpty())
		Expect(info.InstallMethod).To(Equal(instance.InstallMethodHelm))

		cm, err := cluster.Kubectl.CoreV1().ConfigMaps(ns).Get(ctx, helmchart.EpinioInstanceConfigMapName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(cm.Data["id"]).To(Equal(info.ID))
		Expect(cm.Data["installMethod"]).To(Equal("helm"))
		Expect(cm.Labels["app.kubernetes.io/name"]).To(Equal("epinio-server"))
		Expect(cm.Labels["app.kubernetes.io/component"]).To(Equal("epinio"))
		Expect(cm.Labels["epinio.io/instance-metadata"]).To(Equal("true"))
	})

	It("Create returns the existing identity when the ConfigMap already exists", func() {
		first, err := instance.Create(ctx, cluster)
		Expect(err).ToNot(HaveOccurred())

		second, err := instance.Create(ctx, cluster)
		Expect(err).ToNot(HaveOccurred())
		Expect(second).To(Equal(first))
	})

	It("Get reads an existing identity and never replaces it", func() {
		_, err := cluster.Kubectl.CoreV1().ConfigMaps(ns).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      helmchart.EpinioInstanceConfigMapName,
				Namespace: ns,
			},
			Data: map[string]string{
				"id":            "fixed-id-1111",
				"installMethod": "cli",
			},
		}, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		info, err := instance.Get(ctx, cluster)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.ID).To(Equal("fixed-id-1111"))
		Expect(info.InstallMethod).To(Equal("cli"))
		Expect(info.Complete()).To(BeTrue())

		cached, ok := instance.GetCached()
		Expect(ok).To(BeTrue())
		Expect(cached).To(Equal(info))
	})

	It("Get returns an incomplete identity without caching it", func() {
		_, err := cluster.Kubectl.CoreV1().ConfigMaps(ns).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      helmchart.EpinioInstanceConfigMapName,
				Namespace: ns,
			},
			Data: map[string]string{"installMethod": "cli"},
		}, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		info, err := instance.Get(ctx, cluster)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.Complete()).To(BeFalse())
		_, ok := instance.GetCached()
		Expect(ok).To(BeFalse())
	})

	It("Backfill adds a missing id without changing installMethod", func() {
		_, err := cluster.Kubectl.CoreV1().ConfigMaps(ns).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      helmchart.EpinioInstanceConfigMapName,
				Namespace: ns,
			},
			Data: map[string]string{
				"installMethod": "cli",
			},
		}, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		info, err := instance.Backfill(ctx, cluster)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.ID).ToNot(BeEmpty())
		Expect(info.InstallMethod).To(Equal("cli"))
	})

	It("Backfill adds a missing installMethod from env/flag", func() {
		viper.Set("install-method", "cli")
		_, err := cluster.Kubectl.CoreV1().ConfigMaps(ns).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      helmchart.EpinioInstanceConfigMapName,
				Namespace: ns,
			},
			Data: map[string]string{
				"id": "already-there",
			},
		}, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		info, err := instance.Backfill(ctx, cluster)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.ID).To(Equal("already-there"))
		Expect(info.InstallMethod).To(Equal("cli"))
	})
})
