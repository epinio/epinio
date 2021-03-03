package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/suse/carrier/deployments"
	"github.com/suse/carrier/internal/interfaces"
	"github.com/suse/carrier/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
)

// CatalogService is a Service created using Service Catalog.
// Implements the Service interface.
type CatalogService struct {
	InstanceName string
	OrgName      string
	Service      string
	Class        string
	Plan         string
	kubeClient   *kubernetes.Cluster
}

func CatalogServiceLookup(kubeClient *kubernetes.Cluster, org, service string) (interfaces.Service, error) {
	instanceName := serviceResourceName(org, service)

	serviceInstanceGVR := schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1beta1",
		Resource: "serviceinstances",
	}

	dynamicClient, err := dynamic.NewForConfig(kubeClient.RestConfig)
	if err != nil {
		return nil, err
	}

	serviceInstance, err := dynamicClient.Resource(serviceInstanceGVR).Namespace(deployments.WorkloadsDeploymentID).
		Get(context.Background(), instanceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	spec := serviceInstance.Object["spec"].(map[string]interface{})
	className := spec["clusterServiceClassExternalName"].(string)
	planName := spec["clusterServicePlanExternalName"].(string)

	return &CatalogService{
		InstanceName: instanceName,
		OrgName:      org,
		Service:      service,
		Class:        className,
		Plan:         planName,
		kubeClient:   kubeClient,
	}, nil
}

func CreateCatalogService(kubeClient *kubernetes.Cluster, name, org, class, plan string, parameters map[string]string) (interfaces.Service, error) {
	resourceName := serviceResourceName(org, name)

	param, err := json.Marshal(parameters)
	if err != nil {
		return nil, err
	}

	data := fmt.Sprintf(`{
		"apiVersion": "servicecatalog.k8s.io/v1beta1",
		"kind": "ServiceInstance",
		"metadata": { "name": "%s", "namespace": "%s" },
		"spec": {
			"clusterServiceClassExternalName": "%s",
			"clusterServicePlanExternalName": "%s" },
		"parameters": %s
	}`, resourceName, deployments.WorkloadsDeploymentID, class, plan, param)

	decoderUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err = decoderUnstructured.Decode([]byte(data), nil, obj)
	if err != nil {
		return nil, err
	}

	serviceInstanceGVR := schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1beta1",
		Resource: "serviceinstances",
	}

	dynamicClient, err := dynamic.NewForConfig(kubeClient.RestConfig)
	if err != nil {
		return nil, err
	}

	// todo validations - check service instance existence

	_, err = dynamicClient.Resource(serviceInstanceGVR).Namespace(deployments.WorkloadsDeploymentID).
		Create(context.Background(),
			obj,
			metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// todo : wait for instance to be ready.

	return &CatalogService{
		InstanceName: resourceName,
		OrgName:      org,
		Service:      name,
		Class:        class,
		Plan:         plan,
		kubeClient:   kubeClient,
	}, nil
}

func (s *CatalogService) Name() string {
	return s.Service
}

func (s *CatalogService) Org() string {
	return s.OrgName
}

// GetBinding returns an application-specific secret for the service to be
// bound to that application.
func (s *CatalogService) GetBinding(appName string) (*corev1.Secret, error) {
	// TODO Label the secret

	bindingName := bindingResourceName(s.OrgName, s.Service, appName)

	binding, err := s.LookupBinding(bindingName)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		_, err = s.CreateBinding(bindingName, s.OrgName, s.Service)
		if err != nil {
			return nil, err
		}
	}

	return s.GetBindingSecret(bindingName)
}

// LookupBinding finds a ServiceBinding object for the application with Name
// appName if there is one.
func (s *CatalogService) LookupBinding(bindingName string) (interface{}, error) {
	serviceBindingGVR := schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1beta1",
		Resource: "servicebindings",
	}

	dynamicClient, err := dynamic.NewForConfig(s.kubeClient.RestConfig)
	if err != nil {
		return nil, err
	}

	serviceBinding, err := dynamicClient.Resource(serviceBindingGVR).Namespace(deployments.WorkloadsDeploymentID).
		Get(context.Background(), bindingName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	return serviceBinding, nil
}

// CreateBinding creates a ServiceBinding for the application with name appName.
func (s *CatalogService) CreateBinding(bindingName, org, serviceName string) (interface{}, error) {
	serviceInstanceName := serviceResourceName(org, serviceName)

	data := fmt.Sprintf(`{
		"apiVersion": "servicecatalog.k8s.io/v1beta1",
		"kind": "ServiceBinding",
		"metadata": { "name": "%s", "namespace": "%s" },
		"spec": {
				"instanceRef": { "name": "%s" },
				"secretName": "%s" }
	}`, bindingName, deployments.WorkloadsDeploymentID, serviceInstanceName, bindingName)

	decoderUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err := decoderUnstructured.Decode([]byte(data), nil, obj)
	if err != nil {
		return nil, err
	}

	serviceBindingGVR := schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1beta1",
		Resource: "servicebindings",
	}

	dynamicClient, err := dynamic.NewForConfig(s.kubeClient.RestConfig)
	if err != nil {
		return nil, err
	}

	return dynamicClient.Resource(serviceBindingGVR).Namespace(deployments.WorkloadsDeploymentID).
		Create(context.Background(), obj, metav1.CreateOptions{})
}

// GetBindingSecret creates a ServiceBinding for the application with name appName.
func (s *CatalogService) GetBindingSecret(bindingName string) (*corev1.Secret, error) {
	// TODO: Replace hardcoded timeout with a constant
	return s.kubeClient.WaitForSecret(deployments.WorkloadsDeploymentID, bindingName, time.Second*300)
}

// DeleteBinding deletes the ServiceBinding resource. The relevant secret will
// also be deleted automatically.
func (s *CatalogService) DeleteBinding(appName string) error {
	return errors.New("to be implemented")
}

func (s *CatalogService) Delete() error {
	// TODO delete catalog service via service catalog
	return nil
}
