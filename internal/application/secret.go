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
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// loadOrCreateSecret locates and returns the kube secret storing the referenced
// application's resource. If necessary it creates that secret.
func loadOrCreateSecret(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, secretName, areaLabel string) (*v1.Secret, error) {
	secret, err := cluster.GetSecret(ctx, appRef.Namespace, secretName)
	if err != nil {
		// If error is `Not Found`. Create the secret.
		if apierrors.IsNotFound(err) {
			return createSecret(ctx, cluster, appRef, secretName, areaLabel)
		}
		return nil, errors.Wrapf(err, "error getting secret %s", secretName)
	}
	return secret, nil
}

// createSecret will create the secret in the cluster
func createSecret(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, secretName, areaLabel string) (*v1.Secret, error) {
	app, err := Get(ctx, cluster, appRef)
	if err != nil {
		// Should not happen. Application was validated to exist
		// already somewhere by callers.
		return nil, errors.Wrapf(err, "error getting application resource")
	}

	secret := makeSecret(appRef, areaLabel)
	secret.ObjectMeta.Name = secretName

	ownerReference := makeOwnerReference(app)
	secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ownerReference}

	err = cluster.CreateSecret(ctx, appRef.Namespace, secret)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating secret %s", secretName)
	}
	return &secret, nil
}

// makeOwnerReference creates an OwnerReference
func makeOwnerReference(app *unstructured.Unstructured) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: app.GetAPIVersion(),
		Kind:       app.GetKind(),
		Name:       app.GetName(),
		UID:        app.GetUID(),
	}
}

// makeSecret create a new secret
func makeSecret(appRef models.AppRef, areaLabel string) v1.Secret {
	return v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: appRef.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       appRef.Name,
				"app.kubernetes.io/part-of":    appRef.Namespace,
				"app.kubernetes.io/managed-by": "epinio",
				"app.kubernetes.io/component":  "application",
				EpinioApplicationAreaLabel:     areaLabel,
			},
		},
	}
}
