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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChartsInUse returns a set of app chart names that are referenced by at least
// one application across all namespaces. The map value is always true; absence
// means the chart is not in use.
func ChartsInUse(ctx context.Context, cluster *kubernetes.Cluster) (map[string]bool, error) {
	client, err := cluster.ClientApp()
	if err != nil {
		return nil, err
	}

	list, err := client.Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := map[string]bool{}
	for _, appCR := range list.Items {
		chartName, err := AppChart(&appCR)
		if err != nil || chartName == "" {
			continue
		}
		result[chartName] = true
	}

	return result, nil
}
