package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/interfaces"
	"github.com/epinio/epinio/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/wait"
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

// ServiceClass is a service class managed by Service catalog
type ServiceClass struct {
	Hash        string
	Name        string
	Broker      string
	Description string
	kubeClient  *kubernetes.Cluster
}

type ServiceClassList []ServiceClass

// ServicePlan is a service plan managed by Service catalog
type ServicePlan struct {
	Name        string
	Description string
	Free        bool
}

type ServicePlanList []ServicePlan

// ListPlans returns a ServicePlanList of all available catalog service plans, for the named class
func (sc *ServiceClass) ListPlans() (ServicePlanList, error) {
	servicePlanGVR := schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1beta1",
		Resource: "clusterserviceplans",
	}

	dynamicClient, err := dynamic.NewForConfig(sc.kubeClient.RestConfig)
	if err != nil {
		return nil, err
	}

	labelSelector := fmt.Sprintf("servicecatalog.k8s.io/spec.clusterServiceClassRef.name=%s", sc.Hash)

	servicePlans, err := dynamicClient.Resource(servicePlanGVR).
		List(context.Background(),
			metav1.ListOptions{
				LabelSelector: labelSelector,
			})

	if err != nil {
		return nil, err
	}

	result := ServicePlanList{}

	for _, servicePlan := range servicePlans.Items {
		spec := servicePlan.Object["spec"].(map[string]interface{})

		externalName := spec["externalName"].(string)
		description := spec["description"].(string)
		isAFreePlan := spec["free"].(bool)

		result = append(result, ServicePlan{
			Name:        externalName,
			Description: description,
			Free:        isAFreePlan,
		})
	}

	return result, nil
}

// ListClasses returns a ServiceClassList of all available catalog service classes
func ListClasses(kubeClient *kubernetes.Cluster) (ServiceClassList, error) {

	serviceClassGVR := schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1beta1",
		Resource: "clusterserviceclasses",
	}

	dynamicClient, err := dynamic.NewForConfig(kubeClient.RestConfig)
	if err != nil {
		return nil, err
	}

	serviceClasses, err := dynamicClient.Resource(serviceClassGVR).
		List(context.Background(),
			metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	result := ServiceClassList{}

	for _, serviceClass := range serviceClasses.Items {
		spec := serviceClass.Object["spec"].(map[string]interface{})

		// We have brokers (google :( ) which use unfriendly
		// service names (i.e. some kind of hash/uuid). The
		// external name is where they store a nice name.  And
		// brokers with an ok name have the same in the
		// external name also.
		//
		// Consequence of hiding the hash/uuid name from the
		// user here: `ClassLookup` finds a class by listing
		// all and filtering, instead of `get`ing it directly
		// by its name.

		externalName := spec["externalName"].(string)
		description := spec["description"].(string)
		clusterServiceBrokerName := spec["clusterServiceBrokerName"].(string)

		metadata := serviceClass.Object["metadata"].(map[string]interface{})

		labels := metadata["labels"].(map[string]interface{})
		hash := labels["servicecatalog.k8s.io/spec.externalID"].(string)

		result = append(result, ServiceClass{
			Name:        externalName,
			Broker:      clusterServiceBrokerName,
			Description: description,
			Hash:        hash,
			kubeClient:  kubeClient,
		})
	}

	return result, nil
}

func ClassLookup(kubeClient *kubernetes.Cluster, serviceClassName string) (*ServiceClass, error) {
	serviceClassGVR := schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1beta1",
		Resource: "clusterserviceclasses",
	}

	dynamicClient, err := dynamic.NewForConfig(kubeClient.RestConfig)
	if err != nil {
		return nil, err
	}

	// We always list and then filter for the external name of the
	// class.  See `ListClasses` above on why.
	//
	// Note that there are no labels enabling easy filtering by
	// kube itself, so it is done here in code. Like
	// `ServiceClassMatching` (pass/client.go), just for exact
	// match.

	serviceClasses, err := dynamicClient.Resource(serviceClassGVR).
		List(context.Background(),
			metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	for _, serviceClass := range serviceClasses.Items {
		spec := serviceClass.Object["spec"].(map[string]interface{})

		externalName := spec["externalName"].(string)

		if externalName != serviceClassName {
			continue
		}

		description := spec["description"].(string)
		clusterServiceBrokerName := spec["clusterServiceBrokerName"].(string)

		metadata := serviceClass.Object["metadata"].(map[string]interface{})

		labels := metadata["labels"].(map[string]interface{})
		hash := labels["servicecatalog.k8s.io/spec.externalID"].(string)

		return &ServiceClass{
			Name:        externalName,
			Broker:      clusterServiceBrokerName,
			Description: description,
			Hash:        hash,
			kubeClient:  kubeClient,
		}, nil
	}

	// Not found
	return nil, nil
}

// CatalogServiceList returns a ServiceList of all available catalog Services
func CatalogServiceList(kubeClient *kubernetes.Cluster, org string) (interfaces.ServiceList, error) {
	labelSelector := fmt.Sprintf("app.kubernetes.io/name=epinio, epinio.suse.org/organization=%s", org)

	serviceInstanceGVR := schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1beta1",
		Resource: "serviceinstances",
	}

	dynamicClient, err := dynamic.NewForConfig(kubeClient.RestConfig)
	if err != nil {
		return nil, err
	}

	serviceInstances, err := dynamicClient.Resource(serviceInstanceGVR).
		Namespace(deployments.WorkloadsDeploymentID).
		List(context.Background(),
			metav1.ListOptions{
				LabelSelector: labelSelector,
			})

	if err != nil {
		return nil, err
	}

	result := interfaces.ServiceList{}

	for _, serviceInstance := range serviceInstances.Items {
		spec := serviceInstance.Object["spec"].(map[string]interface{})
		className := spec["clusterServiceClassExternalName"].(string)
		planName := spec["clusterServicePlanExternalName"].(string)

		metadata := serviceInstance.Object["metadata"].(map[string]interface{})
		instanceName := metadata["name"].(string)
		labels := metadata["labels"].(map[string]interface{})
		org := labels["epinio.suse.org/organization"].(string)
		service := labels["epinio.suse.org/service"].(string)

		result = append(result, &CatalogService{
			InstanceName: instanceName,
			OrgName:      org,
			Service:      service,
			Class:        className,
			Plan:         planName,
			kubeClient:   kubeClient,
		})
	}

	return result, nil
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
		"metadata": {
			"name": "%s",
			"namespace": "%s",
			"labels": {
				"epinio.suse.org/service-type": "catalog",
				"epinio.suse.org/service":      "%s",
				"epinio.suse.org/organization": "%s",
				"app.kubernetes.io/name":        "epinio"
			}
		},
		"spec": {
			"clusterServiceClassExternalName": "%s",
			"clusterServicePlanExternalName": "%s" },
		"parameters": %s
	}`, resourceName, deployments.WorkloadsDeploymentID,
		name, org, class, plan, param)

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
		_, err = s.CreateBinding(bindingName, s.OrgName, s.Service, appName)
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
func (s *CatalogService) CreateBinding(bindingName, org, serviceName, appName string) (interface{}, error) {
	serviceInstanceName := serviceResourceName(org, serviceName)

	data := fmt.Sprintf(`{
		"apiVersion": "servicecatalog.k8s.io/v1beta1",
		"kind": "ServiceBinding",
		"metadata": { 
			"name": "%s", 
			"namespace": "%s",
		    "labels": { 
				"app.kubernetes.io/name": "%s",
				"app.kubernetes.io/part-of": "%s",
				"app.kubernetes.io/component": "servicebinding",
				"app.kubernetes.io/managed-by": "epinio"
			}
		},
		"spec": {
			"instanceRef": { "name": "%s" },
			"secretName": "%s" 
		}
	}`, bindingName, deployments.WorkloadsDeploymentID, appName, org, serviceInstanceName, bindingName)

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

	serviceBinding, err := dynamicClient.Resource(serviceBindingGVR).Namespace(deployments.WorkloadsDeploymentID).
		Create(context.Background(), obj, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// Update the binding secret with kubernetes app labels
	secret, err := s.GetBindingSecret(bindingName)
	if err != nil {
		return nil, err
	}

	labels := secret.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["app.kubernetes.io/name"] = appName
	labels["app.kubernetes.io/part-of"] = org
	labels["app.kubernetes.io/component"] = "servicebindingsecret"
	labels["app.kubernetes.io/managed-by"] = "epinio"
	secret.SetLabels(labels)

	_, err = s.kubeClient.Kubectl.CoreV1().Secrets(deployments.WorkloadsDeploymentID).Update(context.Background(), secret, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return serviceBinding, nil
}

// GetBindingSecret returns the Secret that represents the binding of a Service
// to an Application.
func (s *CatalogService) GetBindingSecret(bindingName string) (*corev1.Secret, error) {
	return s.kubeClient.WaitForSecret(deployments.WorkloadsDeploymentID, bindingName,
		duration.ToServiceSecret())
}

// DeleteBinding deletes the ServiceBinding resource. The relevant secret will
// also be deleted automatically.
func (s *CatalogService) DeleteBinding(appName string) error {
	bindingName := bindingResourceName(s.OrgName, s.Service, appName)

	serviceBindingGVR := schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1beta1",
		Resource: "servicebindings",
	}

	dynamicClient, err := dynamic.NewForConfig(s.kubeClient.RestConfig)
	if err != nil {
		return err
	}

	return dynamicClient.Resource(serviceBindingGVR).Namespace(deployments.WorkloadsDeploymentID).
		Delete(context.Background(), bindingName, metav1.DeleteOptions{})
}

func (s *CatalogService) Delete() error {
	serviceInstanceGVR := schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1beta1",
		Resource: "serviceinstances",
	}

	dynamicClient, err := dynamic.NewForConfig(s.kubeClient.RestConfig)
	if err != nil {
		return err
	}

	return dynamicClient.Resource(serviceInstanceGVR).Namespace(deployments.WorkloadsDeploymentID).
		Delete(context.Background(), s.InstanceName, metav1.DeleteOptions{})
}

func (s *CatalogService) Status() (string, error) {
	serviceInstanceGVR := schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1beta1",
		Resource: "serviceinstances",
	}

	dynamicClient, err := dynamic.NewForConfig(s.kubeClient.RestConfig)
	if err != nil {
		return "", err
	}

	serviceInstance, err := dynamicClient.Resource(serviceInstanceGVR).Namespace(deployments.WorkloadsDeploymentID).
		Get(context.Background(), s.InstanceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "Not Found", nil
		} else {
			return "", err
		}
	}

	status := serviceInstance.Object["status"].(map[string]interface{})
	provisioned := status["provisionStatus"].(string)

	return provisioned, nil
}

func (s *CatalogService) WaitForProvision() error {
	serviceInstanceGVR := schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1beta1",
		Resource: "serviceinstances",
	}

	dynamicClient, err := dynamic.NewForConfig(s.kubeClient.RestConfig)
	if err != nil {
		return err
	}

	namespace := dynamicClient.Resource(serviceInstanceGVR).Namespace(deployments.WorkloadsDeploymentID)

	return wait.PollImmediate(time.Second, duration.ToServiceProvision(), func() (bool, error) {
		serviceInstance, err := namespace.Get(context.Background(), s.InstanceName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, errors.New("Not Found")
			}
			return false, err
		}

		status, ok := serviceInstance.Object["status"].(map[string]interface{})
		if !ok {
			return false, nil
		}

		provisioned, ok := status["provisionStatus"].(string)
		if !ok {
			return false, nil
		}

		return provisioned == "Provisioned", nil
	})
}

func (s *CatalogService) Details() (map[string]string, error) {
	details := map[string]string{}

	details["Class"] = s.Class
	details["Plan"] = s.Plan

	return details, nil
}
