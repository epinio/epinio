package k3s

import (
	"context"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes/platform/generic"
	"github.com/kyokomi/emoji"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type K3s struct {
	generic.Generic
}

func (k *K3s) Describe() string {
	return emoji.Sprintf(":anchor:Detected kubernetes platform: %s\n:earth_americas:ExternalIPs: %s\n:curly_loop:InternalIPs: %s", k.String(), k.ExternalIPs(), k.InternalIPs)
}

func (k *K3s) String() string { return "k3s" }

func (k *K3s) Detect(ctx context.Context, kube *kubernetes.Clientset) bool {
	nodes, err := kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false
	}
	for _, n := range nodes.Items {
		if strings.Contains(n.Spec.ProviderID, "k3s://") {
			return true
		}
	}
	return false
}

func (k *K3s) ExternalIPs() []string {
	return k.InternalIPs
}

func NewPlatform() *K3s {
	return &K3s{}
}
