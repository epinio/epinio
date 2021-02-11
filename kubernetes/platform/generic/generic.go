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

func (k *Generic) Detect(kube *kubernetes.Clientset) bool {
	return false
}

func (k *Generic) Load(kube *kubernetes.Clientset) error {
	nodes, err := kube.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
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
