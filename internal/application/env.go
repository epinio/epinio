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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// EnvironmentNames returns the names of all environment variables which are set on the named
// application by users.  It does not return values.
func EnvironmentNames(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) ([]string, error) {
	evSecret, err := envLoad(ctx, cluster, appRef)
	if err != nil {
		return nil, err
	}

	result := []string{}
	for name := range evSecret.Data {
		result = append(result, name)
	}

	return result, nil
}

// Environment returns the environment variables and their values which are set on the named
// application by users
func Environment(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (models.EnvVariableMap, error) {
	evSecret, err := envLoad(ctx, cluster, appRef)
	if err != nil {
		return nil, err
	}

	return EnvironmentFromSecret(evSecret), nil
}

// EnvironmentFromSecret is the core of Environment, extracting the set of environment variable
// assignments from the secret containing them.
func EnvironmentFromSecret(evSecret *v1.Secret) models.EnvVariableMap {
	result := models.EnvVariableMap{}
	for name, value := range evSecret.Data {
		result[name] = string(value)
	}

	return result
}

// EnvironmentSet adds or modifies the specified environment variable
// for the named application. When the function returns the variable
// will have the specified value. If the application is active the
// workload is restarted to update it to the new settings. The
// function will __not__ wait on this to complete.
func EnvironmentSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, assignments models.EnvVariableMap, replace bool) error {
	return envUpdate(ctx, cluster, appRef, func(evSecret *v1.Secret) {
		// Replacement is adding to a clear structure
		if replace {
			evSecret.Data = make(map[string][]byte)
		}
		for name, value := range assignments {
			evSecret.Data[name] = []byte(value)
		}
	})
}

// EnvironmentUnset removes the specified environment variable from the
// named application. When the function returns the variable will be
// gone. If the application is active the workload is restarted to
// update it to the new settings. The function will __not__ wait on
// this to complete.
func EnvironmentUnset(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, varName string) error {
	return envUpdate(ctx, cluster, appRef, func(evSecret *v1.Secret) {
		delete(evSecret.Data, varName)
	})
}

// envUpdate is the helper for the public function encapsulating the
// read/modify/write cycle necessary to update the application's kube
// resource holding the application's environment, and the logic to
// restart the workload so that it may gain the changed settings.
func envUpdate(ctx context.Context, cluster *kubernetes.Cluster,
	appRef models.AppRef, modifyEnvironment func(*v1.Secret)) error {

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		evSecret, err := envLoad(ctx, cluster, appRef)
		if err != nil {
			return err
		}

		if evSecret.Data == nil {
			evSecret.Data = make(map[string][]byte)
		}

		modifyEnvironment(evSecret)

		_, err = cluster.Kubectl.CoreV1().Secrets(appRef.Namespace).Update(
			ctx, evSecret, metav1.UpdateOptions{})

		return err
	})
}

// envLoad locates and returns the kube secret storing the referenced
// application's environment. If necessary it creates that secret.
func envLoad(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*v1.Secret, error) {
	secretName := appRef.MakeEnvSecretName()
	return loadOrCreateSecret(ctx, cluster, appRef, secretName, "environment")
}
