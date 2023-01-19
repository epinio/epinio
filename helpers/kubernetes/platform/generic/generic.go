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

package generic

import (
	"context"

	"github.com/kyokomi/emoji"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Generic struct {
	InternalIPs, ExternalIP []string
}

func (k *Generic) Describe() string {
	return emoji.Sprintf(":anchor:Detected kubernetes platform: %s\n:earth_americas:ExternalIPs: %s\n:curly_loop:InternalIPs: %s", k.String(), k.ExternalIPs(), k.InternalIPs)
}

func (k *Generic) String() string { return "generic" }

func (k *Generic) Detect(ctx context.Context, kube *kubernetes.Clientset) bool {
	return false
}

func (k *Generic) Load(ctx context.Context, kube *kubernetes.Clientset) error {
	nodes, err := kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	// See also https://github.com/kubernetes/kubernetes/blob/47943d5f9ce7dbe8fbf805ff76a5eb9726c6af0c/test/e2e/framework/util.go#L1266
	internalIPs := []string{}
	externalIPs := []string{}
	for _, n := range nodes.Items {
		for _, address := range n.Status.Addresses {
			switch address.Type {
			case "InternalIP":
				internalIPs = append(internalIPs, address.Address)
			case "ExternalIP":
				externalIPs = append(externalIPs, address.Address)
			}
		}
	}
	k.InternalIPs = internalIPs
	k.ExternalIP = externalIPs

	return nil
}

func (k *Generic) ExternalIPs() []string {
	return k.ExternalIP
}

func NewPlatform() *Generic {
	return &Generic{}
}
