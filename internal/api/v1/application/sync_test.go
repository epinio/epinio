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

package application

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("isPodReady", func() {

	It("is true when at least one container is ready", func() {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Ready: false},
					{Ready: true},
				},
			},
		}
		Expect(isPodReady(pod)).To(BeTrue())
	})

	It("is false when no container is ready", func() {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Ready: false},
				},
			},
		}
		Expect(isPodReady(pod)).To(BeFalse())
	})

	It("is false when there are no container statuses", func() {
		pod := &corev1.Pod{}
		Expect(isPodReady(pod)).To(BeFalse())
	})
})

var _ = Describe("buildSyncCommand", func() {

	It("targets the app source directory in files mode", func() {
		cmd := buildSyncCommand("files", "", "")
		Expect(cmd).To(HaveLen(5))
		Expect(cmd[0]).To(Equal("sh"))
		Expect(cmd[2]).To(ContainSubstring("chmod -R u+w"))
		Expect(cmd[2]).To(ContainSubstring("tar xf - -C"))
		Expect(cmd[2]).To(ContainSubstring(`kill -9`))
		Expect(cmd[4]).To(Equal("/workspace/source/app"))
	})

	It("honors a custom destination in files mode", func() {
		cmd := buildSyncCommand("files", "/srv/app dir", "")
		Expect(cmd[4]).To(Equal("/srv/app dir"))
	})

	It("moves the binary to the sync directory in binary mode", func() {
		cmd := buildSyncCommand("binary", "", "")
		Expect(cmd).To(HaveLen(6))
		Expect(cmd[2]).To(ContainSubstring("tar xf - -C /tmp"))
		Expect(cmd[2]).To(ContainSubstring(`kill -9`))
		Expect(cmd[4]).To(Equal("app"))
		Expect(cmd[5]).To(Equal("/epinio-sync/app"))
	})

	It("honors binary name and destination overrides", func() {
		cmd := buildSyncCommand("binary", "/srv/bin/my app", "my-binary")
		Expect(cmd[4]).To(Equal("my-binary"))
		Expect(cmd[5]).To(Equal("/srv/bin/my app"))
	})

	It("returns nil for an unknown mode", func() {
		Expect(buildSyncCommand("bogus", "", "")).To(BeNil())
	})
})

var _ = Describe("findReadyPod", func() {

	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	makePod := func(name, appName string, ready bool) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "workspace",
				Labels: map[string]string{
					"app.kubernetes.io/name": appName,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: appName}},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Ready: ready},
				},
			},
		}
	}

	It("returns the ready pod and its first container", func() {
		cluster := &kubernetes.Cluster{
			Kubectl: k8sfake.NewSimpleClientset(
				makePod("myapp-pod-1", "myapp", false),
				makePod("myapp-pod-2", "myapp", true),
			),
		}

		podName, containerName, apiError := findReadyPod(
			ctx, cluster, "workspace", "myapp",
		)
		Expect(apiError).To(BeNil())
		Expect(*podName).To(Equal("myapp-pod-2"))
		Expect(*containerName).To(Equal("myapp"))
	})

	It("ignores pods belonging to other apps", func() {
		cluster := &kubernetes.Cluster{
			Kubectl: k8sfake.NewSimpleClientset(
				makePod("other-pod", "other", true),
			),
		}

		_, _, apiError := findReadyPod(ctx, cluster, "workspace", "myapp")
		Expect(apiError).ToNot(BeNil())
	})

	It("fails with 503 when no pod is ready", func() {
		cluster := &kubernetes.Cluster{
			Kubectl: k8sfake.NewSimpleClientset(
				makePod("myapp-pod-1", "myapp", false),
			),
		}

		_, _, apiError := findReadyPod(ctx, cluster, "workspace", "myapp")
		Expect(apiError).ToNot(BeNil())
		Expect(apiError.FirstStatus()).To(Equal(503))
	})
})

var _ = Describe("swapPodImage", func() {

	var ctx context.Context
	var appRef models.AppRef

	BeforeEach(func() {
		ctx = context.Background()
		appRef = models.NewAppRef("myapp", "workspace")
	})

	makeDeployment := func() *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myapp-deployment",
				Namespace: "workspace",
				Labels: map[string]string{
					"app.kubernetes.io/name": "myapp",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "myapp", Image: "old-image"},
						},
					},
				},
			},
		}
	}

	makeRunningPod := func() *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myapp-pod",
				Namespace: "workspace",
				Labels: map[string]string{
					"app.kubernetes.io/name": "myapp",
				},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}
	}

	It("patches the deployment with the supervisor wrapper", func() {
		clientset := k8sfake.NewSimpleClientset(
			makeDeployment(),
			makeRunningPod(),
		)
		cluster := &kubernetes.Cluster{Kubectl: clientset}

		swapError := swapPodImage(
			ctx, cluster, appRef, "new-image", 1001, 1000, "",
		)
		Expect(swapError).ToNot(HaveOccurred())

		patched, getError := clientset.AppsV1().
			Deployments("workspace").
			Get(ctx, "myapp-deployment", metav1.GetOptions{})
		Expect(getError).ToNot(HaveOccurred())

		container := patched.Spec.Template.Spec.Containers[0]
		Expect(container.Image).To(Equal("new-image"))
		Expect(container.Command).To(HaveLen(3))
		Expect(container.Command[2]).To(
			ContainSubstring(`"$APP_CMD" &`),
		)
		Expect(container.Command[2]).To(
			ContainSubstring("/cnb/process/web"),
		)
		Expect(container.Command[2]).To(
			ContainSubstring("/epinio-sync/app"),
		)
		Expect(container.VolumeMounts).To(HaveLen(1))
		Expect(container.VolumeMounts[0].MountPath).To(
			Equal("/epinio-sync"),
		)

		podSpec := patched.Spec.Template.Spec
		Expect(podSpec.Volumes).To(HaveLen(1))
		Expect(podSpec.Volumes[0].EmptyDir).ToNot(BeNil())
		Expect(*podSpec.SecurityContext.RunAsUser).To(Equal(int64(1001)))
		Expect(*podSpec.SecurityContext.RunAsGroup).To(Equal(int64(1000)))
	})

	It("uses the process command override in the supervisor", func() {
		clientset := k8sfake.NewSimpleClientset(makeDeployment())
		cluster := &kubernetes.Cluster{Kubectl: clientset}

		swapError := swapPodImage(
			ctx, cluster, appRef, "new-image", 1001, 1000, "/app/bin/start",
		)
		Expect(swapError).ToNot(HaveOccurred())

		patched, getError := clientset.AppsV1().
			Deployments("workspace").
			Get(ctx, "myapp-deployment", metav1.GetOptions{})
		Expect(getError).ToNot(HaveOccurred())

		command := patched.Spec.Template.Spec.Containers[0].Command[2]
		Expect(command).To(ContainSubstring("/app/bin/start &"))
		Expect(command).ToNot(ContainSubstring(`"$APP_CMD" &`))
	})

	It("deletes the running pod so the new image starts immediately", func() {
		clientset := k8sfake.NewSimpleClientset(
			makeDeployment(),
			makeRunningPod(),
		)
		cluster := &kubernetes.Cluster{Kubectl: clientset}

		swapError := swapPodImage(
			ctx, cluster, appRef, "new-image", 1001, 1000, "",
		)
		Expect(swapError).ToNot(HaveOccurred())

		pods, listError := clientset.CoreV1().
			Pods("workspace").
			List(ctx, metav1.ListOptions{})
		Expect(listError).ToNot(HaveOccurred())
		Expect(pods.Items).To(BeEmpty())
	})

	It("succeeds without a running pod", func() {
		clientset := k8sfake.NewSimpleClientset(makeDeployment())
		cluster := &kubernetes.Cluster{Kubectl: clientset}

		swapError := swapPodImage(
			ctx, cluster, appRef, "new-image", 1001, 1000, "",
		)
		Expect(swapError).ToNot(HaveOccurred())
	})

	It("fails when no deployment matches the app", func() {
		cluster := &kubernetes.Cluster{
			Kubectl: k8sfake.NewSimpleClientset(),
		}

		swapError := swapPodImage(
			ctx, cluster, appRef, "new-image", 1001, 1000, "",
		)
		Expect(swapError).To(HaveOccurred())
		Expect(swapError.Error()).To(
			ContainSubstring("deployment not found"),
		)
	})

	It("fails when the deployment has no containers", func() {
		deployment := makeDeployment()
		deployment.Spec.Template.Spec.Containers = nil
		cluster := &kubernetes.Cluster{
			Kubectl: k8sfake.NewSimpleClientset(deployment),
		}

		swapError := swapPodImage(
			ctx, cluster, appRef, "new-image", 1001, 1000, "",
		)
		Expect(swapError).To(HaveOccurred())
		Expect(swapError.Error()).To(ContainSubstring("no containers"))
	})
})
