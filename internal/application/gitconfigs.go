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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// GitconfigsInUse returns the set of git configuration IDs referenced by at
// least one application across all namespaces. The map value is always true;
// absence means the git configuration is not in use.
//
// It takes the app dynamic client directly (handlers resolve it via
// cluster.ClientApp()) so it can be unit-tested with a fake client.
//
// Note: apps stamp the *selected* configuration on spec.origin.git.gitconfig
// at push time. Apps whose origin is not git, and git apps pushed before the
// gitconfig feature (or against a public repo), carry no value and are
// skipped.
func GitconfigsInUse(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
) (map[string]bool, error) {
	list, err := client.Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := map[string]bool{}
	for i := range list.Items {
		gitconfig, found, nestedError := unstructured.NestedString(
			list.Items[i].Object,
			"spec", "origin", "git", "gitconfig",
		)
		if nestedError != nil || !found || gitconfig == "" {
			continue
		}

		result[gitconfig] = true
	}

	return result, nil
}
