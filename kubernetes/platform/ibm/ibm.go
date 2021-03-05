package ibm

import (
	"context"
	"strings"

	"github.com/kyokomi/emoji"
	"github.com/suse/carrier/kubernetes/platform/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ibm struct {
	generic.Generic
}

func (k *ibm) Describe() string {
	return emoji.Sprintf(":anchor:Detected kubernetes platform: %s\n:earth_americas:ExternalIPs: %s\n:curly_loop:InternalIPs: %s", k.String(), k.ExternalIPs(), k.InternalIPs)
}

func (k *ibm) String() string { return "ibm" }

func (k *ibm) Detect(kube *kubernetes.Clientset) bool {
	nodes, err := kube.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
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

func (k *ibm) ExternalIPs() []string {
	return k.Generic.ExternalIP
}

func NewPlatform() *ibm {
	return &ibm{}
}
