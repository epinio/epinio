package k3s

import (
	"context"
	"strings"

	"github.com/kyokomi/emoji"
	"github.com/suse/carrier/kubernetes/platform/generic"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type k3s struct {
	generic.Generic
}

func (k *k3s) Describe() string {
	return emoji.Sprintf(":anchor:Detected kubernetes platform: %s\n:earth_americas:ExternalIPs: %s\n:curly_loop:InternalIPs: %s", k.String(), k.ExternalIPs(), k.InternalIPs)
}

func (k *k3s) String() string { return "k3s" }

func (k *k3s) Detect(kube *kubernetes.Clientset) bool {
	nodes, err := kube.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
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

func (k *k3s) ExternalIPs() []string {
	return k.InternalIPs
}

func NewPlatform() *k3s {
	return &k3s{}
}
