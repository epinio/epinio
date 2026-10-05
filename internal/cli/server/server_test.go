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

package server

import (
	"context"
	"errors"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/instance"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var _ = Describe("ensureInstanceInfo", func() {
	const ns = "epinio-test"

	var (
		ctx     context.Context
		client  *fake.Clientset
		cluster *kubernetes.Cluster
	)

	createConfigMap := func(data map[string]string) {
		_, err := client.CoreV1().ConfigMaps(ns).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      helmchart.EpinioInstanceConfigMapName,
				Namespace: ns,
			},
			Data: data,
		}, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	BeforeEach(func() {
		instance.ResetCache()
		ctx = context.Background()
		viper.Set("namespace", ns)
		viper.Set("install-method", "helm")

		client = fake.NewSimpleClientset(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		})
		cluster = &kubernetes.Cluster{Kubectl: client}
	})

	AfterEach(func() {
		instance.ResetCache()
		viper.Set("namespace", "")
		viper.Set("install-method", "")
	})

	It("creates the ConfigMap when missing", func() {
		info, err := ensureInstanceInfo(ctx, cluster)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.ID).ToNot(BeEmpty())
		Expect(info.InstallMethod).To(Equal(instance.InstallMethodHelm))

		cm, err := client.CoreV1().ConfigMaps(ns).Get(ctx, helmchart.EpinioInstanceConfigMapName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(cm.Data["id"]).To(Equal(info.ID))
	})

	It("returns a complete identity unchanged", func() {
		createConfigMap(map[string]string{"id": "fixed-id-1111", "installMethod": "cli"})

		info, err := ensureInstanceInfo(ctx, cluster)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.ID).To(Equal("fixed-id-1111"))
		Expect(info.InstallMethod).To(Equal("cli"))
	})

	It("backfills a missing id", func() {
		createConfigMap(map[string]string{"installMethod": "cli"})

		info, err := ensureInstanceInfo(ctx, cluster)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.ID).ToNot(BeEmpty())
		Expect(info.InstallMethod).To(Equal("cli"))

		cm, err := client.CoreV1().ConfigMaps(ns).Get(ctx, helmchart.EpinioInstanceConfigMapName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(cm.Data["id"]).To(Equal(info.ID))
	})

	It("backfills a missing installMethod", func() {
		viper.Set("install-method", "cli")
		createConfigMap(map[string]string{"id": "already-there"})

		info, err := ensureInstanceInfo(ctx, cluster)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.ID).To(Equal("already-there"))
		Expect(info.InstallMethod).To(Equal("cli"))
	})

	It("returns errors other than not-found without creating anything", func() {
		client.PrependReactor("get", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("boom")
		})

		_, err := ensureInstanceInfo(ctx, cluster)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("boom"))

		for _, a := range client.Actions() {
			Expect(a.GetVerb()).ToNot(Equal("create"))
		}
	})
})
