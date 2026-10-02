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

package application_test

import (
	"context"

	kubernetesPkg "github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("listAppDataPVCNames", func() {
	const (
		namespace = "workspace"
		appName   = "myapp"
	)

	var ctx context.Context

	appLabels := map[string]string{
		"app.kubernetes.io/name": appName,
	}

	BeforeEach(func() {
		ctx = context.Background()
	})

	makePVC := func(name string, labels map[string]string) *v1.PersistentVolumeClaim {
		return &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    labels,
			},
		}
	}

	It("returns every PVC for the app, including ordinals left by a scale-down", func() {
		cluster := &kubernetesPkg.Cluster{
			Kubectl: k8sfake.NewSimpleClientset(
				makePVC("stateful-myapp-0", appLabels),
				makePVC("stateful-myapp-1", appLabels),
			),
		}

		names, err := application.ListAppDataPVCNames(ctx, cluster, models.NewAppRef(appName, namespace))
		Expect(err).ToNot(HaveOccurred())
		Expect(names).To(ConsistOf("stateful-myapp-0", "stateful-myapp-1"))
	})

	It("still matches when a claim carries extra labels", func() {
		// Guards the subset match: a chart that adds labels of its own must not
		// fall out of the selector.
		extraLabels := map[string]string{
			"app.kubernetes.io/name":       appName,
			"app.kubernetes.io/component":  "application",
			"app.kubernetes.io/managed-by": "epinio",
		}
		cluster := &kubernetesPkg.Cluster{
			Kubectl: k8sfake.NewSimpleClientset(
				makePVC("stateful-myapp-0", extraLabels),
			),
		}

		names, err := application.ListAppDataPVCNames(ctx, cluster, models.NewAppRef(appName, namespace))
		Expect(err).ToNot(HaveOccurred())
		Expect(names).To(ConsistOf("stateful-myapp-0"))
	})

	It("ignores PVCs for other apps and unlabeled claims", func() {
		otherAppLabels := map[string]string{
			"app.kubernetes.io/name":      "other",
			"app.kubernetes.io/component": "application",
		}
		cluster := &kubernetesPkg.Cluster{
			Kubectl: k8sfake.NewSimpleClientset(
				makePVC("stateful-myapp-0", appLabels),
				makePVC("stateful-other-0", otherAppLabels),
				makePVC("unlabeled-pvc", nil),
			),
		}

		names, err := application.ListAppDataPVCNames(ctx, cluster, models.NewAppRef(appName, namespace))
		Expect(err).ToNot(HaveOccurred())
		Expect(names).To(ConsistOf("stateful-myapp-0"))
	})

	It("returns an empty list when no matching PVCs exist", func() {
		cluster := &kubernetesPkg.Cluster{
			Kubectl: k8sfake.NewSimpleClientset(),
		}

		names, err := application.ListAppDataPVCNames(ctx, cluster, models.NewAppRef(appName, namespace))
		Expect(err).ToNot(HaveOccurred())
		Expect(names).To(BeEmpty())
	})
})
