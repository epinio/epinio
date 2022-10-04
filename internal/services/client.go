package services

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type ServiceClient struct {
	kubeClient        *kubernetes.Cluster
	serviceKubeClient dynamic.NamespaceableResourceInterface
	// COMPATIBILITY SUPPORT for services from before https://github.com/epinio/epinio/issues/1704 fix
	helmChartsKubeClient dynamic.NamespaceableResourceInterface
}

func NewKubernetesServiceClient(kubeClient *kubernetes.Cluster) (*ServiceClient, error) {
	dynamicKubeClient, err := dynamic.NewForConfig(kubeClient.RestConfig)
	if err != nil {
		return nil, err
	}

	serviceGroupVersion := schema.GroupVersionResource{
		Group:    "application.epinio.io",
		Version:  "v1",
		Resource: "services",
	}
	// COMPATIBILITY SUPPORT for services from before https://github.com/epinio/epinio/issues/1704 fix
	helmChartsGroupVersion := schema.GroupVersionResource{
		Group:    "helm.cattle.io",
		Version:  "v1",
		Resource: "helmcharts",
	}

	return &ServiceClient{
		kubeClient:        kubeClient,
		serviceKubeClient: dynamicKubeClient.Resource(serviceGroupVersion),
		// COMPATIBILITY SUPPORT for services from before https://github.com/epinio/epinio/issues/1704 fix
		helmChartsKubeClient: dynamicKubeClient.Resource(helmChartsGroupVersion),
	}, nil
}
