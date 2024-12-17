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

package ibm

import (
	"context"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes/platform/generic"
	"github.com/kyokomi/emoji"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// IBM represents the ibm kubernetes platform.
type IBM struct {
	generic.Generic
}

// Describe returns information about the platform.
func (k *IBM) Describe() string {
	return emoji.Sprintf(":anchor:Detected kubernetes platform: %s\n:earth_americas:ExternalIPs: %s\n:curly_loop:InternalIPs: %s", k.String(), k.ExternalIPs(), k.InternalIPs)
}

func (k *IBM) String() string { return "ibm" }

// Detect detects if it is a ibm platform.
func (k *IBM) Detect(ctx context.Context, kube *kubernetes.Clientset) bool {
	nodes, err := kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false
	}
	for _, n := range nodes.Items {
		if strings.Contains(n.Spec.ProviderID, "ibm://") {
			return true
		}
	}
	return false
}

// ExternalIPs fetches the ibm IP.
func (k *IBM) ExternalIPs() []string {
	return k.Generic.ExternalIP
}

// NewPlatform returns an instance of ibm struct.
func NewPlatform() *IBM {
	return &IBM{}
}
