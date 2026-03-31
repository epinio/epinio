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

// Package namespaces encapsulates all the functionality around Epinio-controlled namespaces
// TODO: Consider moving this + the applications + the configurations packages under
// "models".
package namespaces

import (
	"context"
	"fmt"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/registry"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Namespace represents an epinio-controlled namespace in the system
type Namespace struct {
	Name      string
	CreatedAt metav1.Time
}

func (n Namespace) Namespace() string {
	return n.Name
}

func List(ctx context.Context, kubeClient *kubernetes.Cluster) ([]Namespace, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: kubernetes.EpinioNamespaceLabelKey + "=" + kubernetes.EpinioNamespaceLabelValue,
	}

	namespaceList, err := kubeClient.Kubectl.CoreV1().Namespaces().List(ctx, listOptions)
	if err != nil {
		return []Namespace{}, err
	}

	result := []Namespace{}
	for _, namespace := range namespaceList.Items {
		result = append(result, Namespace{
			Name:      namespace.Name,
			CreatedAt: namespace.CreationTimestamp,
		})
	}

	return result, nil
}

// Exists checks if the named epinio-controlled namespace exists or
// not, and returns an appropriate boolean flag
func Exists(ctx context.Context, kubeClient *kubernetes.Cluster, lookupNamespace string) (bool, error) {
	namespaces, err := List(ctx, kubeClient)
	if err != nil {
		return false, err
	}
	for _, namespace := range namespaces {
		if namespace.Name == lookupNamespace {
			return true, nil
		}
	}

	return false, nil
}

// Get returns the meta data of  the named epinio-controlled namespace
func Get(ctx context.Context, kubeClient *kubernetes.Cluster, lookupNamespace string) (*Namespace, error) {
	namespaces, err := List(ctx, kubeClient)
	if err != nil {
		return nil, err
	}
	for _, namespace := range namespaces {
		if namespace.Name == lookupNamespace {
			return &namespace, nil
		}
	}

	return nil, nil
}

// Create generates a new epinio-controlled namespace, i.e. a kube
// namespace plus a service account.
func Create(ctx context.Context, kubeClient *kubernetes.Cluster, namespace string) error {
	if _, err := kubeClient.Kubectl.CoreV1().Namespaces().Create(
		ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					kubernetes.EpinioNamespaceLabelKey: kubernetes.EpinioNamespaceLabelValue,
				},
				Annotations: map[string]string{
					"linkerd.io/inject": "enabled",
				},
			},
		},
		metav1.CreateOptions{},
	); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return errors.Errorf("Namespace '%s' name cannot be used. Please try another name", namespace)
		}
		return err
	}

	if err := createServiceAccount(ctx, kubeClient, namespace); err != nil {
		return errors.Wrap(err, "failed to create a service account for apps")
	}

	// Do not tie secret propagation to the client request context. Under CI load,
	// client-side timeouts can cancel the request while Kubernetes controllers are
	// still propagating the secret.
	waitCtx, cancel := context.WithTimeout(context.Background(), duration.ToSecretCopied())
	defer cancel()
	if _, err := kubeClient.WaitForSecret(waitCtx, namespace, "registry-creds", duration.ToSecretCopied()); err != nil {
		// Namespace creation should not block for minutes under API rate limiting.
		// Treat delayed secret propagation as eventually-consistent and let follow-up
		// operations retry while surfacing diagnostics for debugging.
		diagCtx, diagCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer diagCancel()

		var sourceSecretState string
		var targetSecretState string
		var serviceAccountPullSecrets string
		var getErr error

		_, getErr = kubeClient.Kubectl.CoreV1().Secrets("epinio").Get(diagCtx, registry.CredentialsSecretName, metav1.GetOptions{})
		if getErr == nil {
			sourceSecretState = "present"
		} else if apierrors.IsNotFound(getErr) {
			sourceSecretState = "missing"
		} else {
			sourceSecretState = fmt.Sprintf("error: %v", getErr)
		}

		_, getErr = kubeClient.Kubectl.CoreV1().Secrets(namespace).Get(diagCtx, registry.CredentialsSecretName, metav1.GetOptions{})
		if getErr == nil {
			targetSecretState = "present"
		} else if apierrors.IsNotFound(getErr) {
			targetSecretState = "missing"
		} else {
			targetSecretState = fmt.Sprintf("error: %v", getErr)
		}

		sa, saErr := kubeClient.Kubectl.CoreV1().ServiceAccounts(namespace).Get(diagCtx, namespace, metav1.GetOptions{})
		if saErr == nil {
			serviceAccountPullSecrets = fmt.Sprintf("%v", sa.ImagePullSecrets)
		} else {
			serviceAccountPullSecrets = fmt.Sprintf("error: %v", saErr)
		}

		helpers.Logger.Warnw(
			"registry-creds secret not ready after namespace creation; continuing and relying on eventual consistency",
			"namespace", namespace,
			"error", err,
			"sourceSecret", sourceSecretState,
			"targetSecret", targetSecretState,
			"serviceAccountImagePullSecrets", serviceAccountPullSecrets,
		)
	}

	return nil
}

// Delete destroys an epinio-controlled namespace, i.e. the associated
// kube namespace and service account.
func Delete(ctx context.Context, kubeClient *kubernetes.Cluster, namespace string) error {
	err := kubeClient.Kubectl.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return kubeClient.WaitForNamespaceMissing(ctx, nil, namespace, duration.ToNamespaceDeletion())
}

// createServiceAccount is a helper to `Create` which creates the
// service account applications pushed to the namespace need for
// permission handling.
func createServiceAccount(ctx context.Context, kubeClient *kubernetes.Cluster, targetNamespace string) error {
	automountServiceAccountToken := true
	_, err := kubeClient.Kubectl.CoreV1().ServiceAccounts(targetNamespace).Create(
		ctx,
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: targetNamespace,
			},
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: registry.CredentialsSecretName},
			},
			AutomountServiceAccountToken: &automountServiceAccountToken,
		}, metav1.CreateOptions{})

	return err
}
