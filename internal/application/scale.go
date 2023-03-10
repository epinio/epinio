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
	"strconv"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const (
	instanceKey = "desired"
)

// Scaling returns the number of desired instances set by a user for the application
func Scaling(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (int32, error) {
	scaleSecret, err := scaleLoad(ctx, cluster, appRef)
	if err != nil {
		return 0, err
	}

	return ScalingFromSecret(scaleSecret)
}

// ScalingFromSecret is the core of Scaling, extracting the desired number of instances from the
// secret containing them.
func ScalingFromSecret(scaleSecret *v1.Secret) (int32, error) {
	i, err := strconv.ParseInt(string(scaleSecret.Data[instanceKey]), 10, 32)
	if err != nil {
		return 0, err
	}
	result := int32(i)

	// Reject bad values, and assume single instance - Return err better ? Save back, fix resource ?
	if result < 0 {
		result = 1
	}

	return result, nil
}

// ScalingSet sets the desired number of instances for the named application.
// When the function returns the number is saved.
func ScalingSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, instances int32) error {
	return scaleUpdate(ctx, cluster, appRef, func(scaleSecret *v1.Secret) {
		scaleSecret.Data[instanceKey] = []byte(strconv.Itoa(int(instances)))
	})
}

// scaleUpdate is a helper for the public functions. It encapsulates the read/modify/write cycle
// necessary to update the application's kube resource holding the application's number of desired
// instances
func scaleUpdate(ctx context.Context, cluster *kubernetes.Cluster,
	appRef models.AppRef, modifyScaling func(*v1.Secret)) error {

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		scaleSecret, err := scaleLoad(ctx, cluster, appRef)
		if err != nil {
			return err
		}

		if scaleSecret.Data == nil {
			scaleSecret.Data = map[string][]byte{
				instanceKey: []byte(`1`),
			}
		}

		modifyScaling(scaleSecret)

		_, err = cluster.Kubectl.CoreV1().Secrets(appRef.Namespace).Update(
			ctx, scaleSecret, metav1.UpdateOptions{})

		return err
	})
}

// scaleLoad locates and returns the kube secret storing the referenced application's desired number of
// instances. If necessary it creates that secret.
func scaleLoad(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*v1.Secret, error) {
	secretName := appRef.MakeScaleSecretName()
	return loadOrCreateSecret(ctx, cluster, appRef, secretName, "scaling")
}
