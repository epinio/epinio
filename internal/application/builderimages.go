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
	"k8s.io/client-go/dynamic"
)

// BuilderImagesInUse returns a set of builder image references that are used by
// at least one application across all namespaces. The map value is always true;
// absence means the builder image is not in use.
//
// It takes the app dynamic client directly (handlers resolve it via
// cluster.ClientApp()) so it can be unit-tested with a fake client.
//
// Note: apps record the builder *image* string (e.g. paketobuildpacks/...) at
// stage time, not the BuilderImage CR name. Callers therefore match against
// BuilderImage.Image (spec.image), not the CR name.
func BuilderImagesInUse(ctx context.Context, client dynamic.NamespaceableResourceInterface) (map[string]bool, error) {
	list, err := client.Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := map[string]bool{}
	for _, appCR := range list.Items {
		image, err := BuilderURL(&appCR)
		if err != nil || image == "" {
			continue
		}
		result[image] = true
	}

	return result, nil
}
