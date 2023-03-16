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
	"github.com/epinio/epinio/internal/routes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// DesiredRoutes lists all desired routes for the given application
// The list is constructed from the stored information on the
// Application Custom Resource.
func DesiredRoutes(appCR *unstructured.Unstructured) ([]string, error) {
	desiredRoutes, found, err := unstructured.NestedStringSlice(appCR.Object, "spec", "routes")
	if !found {
		// [NO-ROUTES] Not an error. Signal that there are no desired routes.  See `Create`
		// for the converse. An empty slice becomes an omitted field. Same marker as here.
		return []string{}, nil
	}
	if err != nil {
		return []string{}, err
	}

	return desiredRoutes, nil
}

// AddActualApplicationRoutes is a helper for List. It loads all the epinio controlled ingresses in
// the namespace into memory, indexes their routes by namespace and application, and returns the
// resulting map of route lists.  ATTENTION: Using an empty string for the namespace loads the
// information from all namespaces.
func AddActualApplicationRoutes(auxiliary map[ConfigurationKey]AppData, ctx context.Context, cluster *kubernetes.Cluster, namespace string) (map[ConfigurationKey]AppData, error) {
	ingressList, err := cluster.Kubectl.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(map[string]string{
			"app.kubernetes.io/component": "application",
		}).AsSelector().String(),
	})
	if err != nil {
		return nil, err
	}

	for _, ingress := range ingressList.Items {
		routes, err := routes.FromIngress(ingress)
		if err != nil {
			return nil, err
		}

		appName := ingress.Labels["app.kubernetes.io/name"]
		appNamespace := ingress.Labels["app.kubernetes.io/part-of"]
		key := EncodeConfigurationKey(appName, appNamespace)

		if _, found := auxiliary[key]; !found {
			auxiliary[key] = AppData{}
		}

		data := auxiliary[key]

		for _, r := range routes {
			data.routes = append(data.routes, r.String())
		}

		auxiliary[key] = data
	}

	return auxiliary, nil
}

// ListRoutes lists all (currently active) routes for the given application
// The list is constructed from the actual Ingresses and not from the stored
// information on the Application Custom Resource.
func ListRoutes(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) ([]string, error) {
	ingressList, err := ingressListForApp(ctx, cluster, appRef)
	if err != nil {
		return []string{}, err
	}

	result := []string{}
	for _, ingress := range ingressList.Items {
		routes, err := routes.FromIngress(ingress)
		if err != nil {
			return result, err
		}
		for _, r := range routes {
			result = append(result, r.String())
		}
	}

	return result, nil
}

func ingressListForApp(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*networkingv1.IngressList, error) {
	// Find all user credential secrets
	ingressSelector := labels.Set(map[string]string{
		"app.kubernetes.io/name": appRef.Name,
	}).AsSelector().String()

	return cluster.Kubectl.NetworkingV1().Ingresses(appRef.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: ingressSelector,
	})
}
