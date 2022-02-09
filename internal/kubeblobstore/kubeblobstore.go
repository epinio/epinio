// Package kubeblobstore can be used to create Kubernetes Persistent Volume Claims
// that store files.
package kubeblobstore

import (
	"context"
	"fmt"
	"io"

	epiniokubernetes "github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
)

type Blobstore struct {
	restConfig *rest.Config
	namespace  string
}

func NewBlobstore(restConfig *rest.Config, namespace string) Blobstore {
	return Blobstore{
		restConfig: restConfig,
		namespace:  namespace,
	}
}

func (b Blobstore) Clientset() (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(b.restConfig)
}

// Store saves the given file on a new PVC and returns the PVC name.
// The PVC is created on the blobstore namespace.
// TODO: Accept an owner reference for the created PVC
func (b Blobstore) Store(ctx context.Context, file io.Reader) (string, error) {
	pvcName, err := b.newPVC(ctx)
	if err != nil {
		return "", err
	}

	pod, err := b.newHelperPod(ctx, pvcName)
	if err != nil {
		return "", err
	}

	// TODO: Upload the file now

	return "", nil
}

// Delete deletes a PVC with the given name from the blobstore namespace
func (b Blobstore) Delete(pvc string) error {
	return nil
}

// Creates a new PVC with a random name
func (b Blobstore) newPVC(ctx context.Context) (string, error) {
	client, err := b.Clientset()
	if err != nil {
		return "", err
	}

	blobUID, err := randstr.Hex16()

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      blobUID,
			Namespace: b.namespace,
			Labels: map[string]string{
				epiniokubernetes.EpinioPVCLabelKey: epiniokubernetes.EpinioPVCLabelValue,
			},
			OwnerReferences: nil, // TODO: The App crd here
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: corev1.ReadWriteOnce, // ReadWrite
			Resources:   corev1.ResourceRequirements{
				// TODO: Accept a file size?
				// Limits: map[corev1.ResourceName]resource.Quantity{
				// 	"": {
				// 		Format: "",
				// 	},
				// },
				// Requests: map[corev1.ResourceName]resource.Quantity{
				// 	"": {
				// 		Format: "",
				// 	},
				// },
			},
		},
	}
	_, err = client.CoreV1().PersistentVolumeClaims(b.namespace).Create(ctx, &pvc, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return pvc.Name, nil
}

// Creates a new Pod with the specified PVC mounted
// This Pod can be used to copy a file on the PVC.
func (b Blobstore) newHelperPod(ctx context.Context, pvcName string) (string, error) {
	client, err := b.Clientset()
	if err != nil {
		return "", err
	}

	podName := fmt.Sprintf("pvc-upload-%s", pvcName)

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: b.namespace,
			Labels: map[string]string{
				epiniokubernetes.EpinioPVCLabelKey: epiniokubernetes.EpinioPVCLabelValue,
			},
			OwnerReferences: nil, // TODO: The App crd here just in case we have left-overs
		},
		Spec: corev1.PodSpec{
			Volumes: nil,
			Containers: []corev1.Container{
				{
					Name:    "uploader",
					Image:   "busybox",
					Command: []string{"/bin/sh"},
					Args: []string{
						"-c",
						"trap : TERM INT; sleep infinity & wait",
					},
					VolumeMounts: volumeMounts,
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  pointer.Int64(1000),
						RunAsGroup: pointer.Int64(1000),
					},
				},
			},
		},
	}
	_, err = client.CoreV1().Pods(b.namespace).Create(ctx, &pod, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return pvc.Name, nil
}
