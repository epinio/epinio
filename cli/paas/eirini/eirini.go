package eirini

import (
	"code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned"
	"github.com/pkg/errors"
	"github.com/suse/carrier/cli/kubernetes"
)

// NewEiriniKubeClient creates a Kubernetes clientset that can
// reference the Eirini CRDs
func NewEiriniKubeClient(cluster *kubernetes.Cluster) (*versioned.Clientset, error) {
	clientset, err := versioned.NewForConfig(cluster.RestConfig)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create eirini Kubernetes client set")
	}

	return clientset, err
}
