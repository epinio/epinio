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

package minikube

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes/platform/generic"

	"github.com/kyokomi/emoji"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Minikube represents the minikube kubernetes platform.
type Minikube struct {
	generic.Generic
}

// Describe returns information about the platform.
func (m *Minikube) Describe() string {
	return emoji.Sprintf(":anchor:Detected kubernetes platform: %s\n:earth_americas:ExternalIPs: %s\n:curly_loop:InternalIPs: %s", m.String(), m.ExternalIPs(), m.InternalIPs)
}

func (m *Minikube) String() string { return "minikube" }

// Detect detects if it is a minikube platform.
func (m *Minikube) Detect(ctx context.Context, kube *kubernetes.Clientset) bool {
	nodes, err := kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false
	}
	for _, n := range nodes.Items {
		labels := n.GetLabels()
		if _, found := labels["minikube.k8s.io/version"]; found {
			return true
		}
	}
	return false
}

// ExternalIPs fetches the minikube IP.
func (m *Minikube) ExternalIPs() []string {
	return m.Generic.InternalIPs
}

// NewPlatform returns an instance of minikube struct.
func NewPlatform() *Minikube {
	return &Minikube{}
}
