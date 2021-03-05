package kind

import (
	"context"
	"strings"

	"github.com/suse/carrier/kubernetes/platform/generic"

	"github.com/kyokomi/emoji"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type kind struct {
	generic.Generic
}

func (k *kind) Describe() string {
	return emoji.Sprintf(":anchor:Detected kubernetes platform: %s\n:earth_americas:ExternalIPs: %s\n:curly_loop:InternalIPs: %s", k.String(), k.ExternalIPs(), k.InternalIPs)
}

func (k *kind) String() string { return "kind" }

func (k *kind) Detect(kube *kubernetes.Clientset) bool {
	nodes, err := kube.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return false
	}
	for _, n := range nodes.Items {
		if strings.Contains(n.Spec.ProviderID, "kind://") {
			return true
		}
	}
	return false
}

func (k *kind) ExternalIPs() []string {
	return k.Generic.InternalIPs
}

func NewPlatform() *kind {
	return &kind{}
}
