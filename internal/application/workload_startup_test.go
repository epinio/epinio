// Copyright © 2021 - 2026 KRUMWARE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("podStartupMs", func() {
	created := metav1.NewTime(time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC))
	ready := metav1.NewTime(created.Add(2 * time.Second))

	readyPod := func() *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{CreationTimestamp: created},
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: ready,
				}},
			},
		}
	}

	It("returns readyAt minus CreatedAt when ready and no restarts", func() {
		ms := podStartupMs(readyPod(), 0)
		Expect(ms).ToNot(BeNil())
		Expect(*ms).To(Equal(int64(2000)))
	})

	It("returns nil when restarts are greater than zero", func() {
		Expect(podStartupMs(readyPod(), 1)).To(BeNil())
	})

	It("returns nil when the pod is not ready", func() {
		pod := readyPod()
		pod.Status.Conditions[0].Status = corev1.ConditionFalse
		Expect(podStartupMs(pod, 0)).To(BeNil())
	})
})
