package services

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type ServiceClient struct {
	kubeClient        *kubernetes.Cluster
	serviceKubeClient dynamic.NamespaceableResourceInterface
}

func NewKubernetesServiceClient(kubeClient *kubernetes.Cluster) (*ServiceClient, error) {
	dynamicKubeClient, err := dynamic.NewForConfig(kubeClient.RestConfig)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:    "application.epinio.io",
		Version:  "v1",
		Resource: "services",
	}

	return &ServiceClient{
		kubeClient:        kubeClient,
		serviceKubeClient: dynamicKubeClient.Resource(gvr),
	}, nil
}
