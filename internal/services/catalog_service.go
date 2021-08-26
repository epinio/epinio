package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/interfaces"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/wait"
)

// CatalogService is a Service created using Service Catalog.
// Implements the Service interface.
type CatalogService struct {
	InstanceName string
	OrgName      string
	Service      string
	Class        string
	Plan         string
	cluster      *kubernetes.Cluster
}

var _ interfaces.Service = &CatalogService{}

// ServiceClass represents a service class managed by Service catalog
type ServiceClass struct {
	Hash        string
	Name        string
	Broker      string
	Description string
	cluster     *kubernetes.Cluster
}

type ServiceClassList []ServiceClass

// ServicePlan represents a service plan managed by Service catalog
type ServicePlan struct {
	Name        string
	Description string
	Free        bool
}

// ServicePlanList represents a collection of service plans
type ServicePlanList []ServicePlan

// Implement the Sort interface for service class slices

// Len (Sort interface) returns the length of the ServiceClassList
func (scl ServiceClassList) Len() int {
	return len(scl)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the ServiceClassList
func (scl ServiceClassList) Swap(i, j int) {
	scl[i], scl[j] = scl[j], scl[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the ServiceClassList and returns true if the condition
// holds, and else false.
func (scl ServiceClassList) Less(i, j int) bool {
	return scl[i].Name < scl[j].Name
}

// Implement the Sort interface for service plan slices

// Len (Sort interface) returns the length of the ServicePlanList
func (spl ServicePlanList) Len() int {
	return len(spl)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the ServicePlanList
func (spl ServicePlanList) Swap(i, j int) {
	spl[i], spl[j] = spl[j], spl[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the ServicePlanList and returns true if the condition
// holds, and else false.
func (spl ServicePlanList) Less(i, j int) bool {
	return spl[i].Name < spl[j].Name
}

// LookupPlan returns the named ServicePlan, for the specified class
func (sc *ServiceClass) LookupPlan(ctx context.Context, plan string) (*ServicePlan, error) {
	client, err := sc.cluster.ClientServiceCatalog("clusterserviceplans")
	if err != nil {
		return nil, err
	}

	labelSelector := fmt.Sprintf("servicecatalog.k8s.io/spec.clusterServiceClassRef.name=%s", sc.Hash)

	servicePlans, err := client.List(ctx, metav1.ListOptions{LabelSelector: labelSelector})

	if err != nil {
		return nil, err
	}

	for _, servicePlan := range servicePlans.Items {
		spec := servicePlan.Object["spec"].(map[string]interface{})

		externalName := spec["externalName"].(string)

		if externalName != plan {
			continue
		}

		description := spec["description"].(string)
		isAFreePlan := spec["free"].(bool)

		return &ServicePlan{
			Name:        externalName,
			Description: description,
			Free:        isAFreePlan,
		}, nil
	}

	return nil, nil
}

// ListPlans returns a ServicePlanList of all available catalog service plans, for the named class
func (sc *ServiceClass) ListPlans(ctx context.Context) (ServicePlanList, error) {
	client, err := sc.cluster.ClientServiceCatalog("clusterserviceplans")
	if err != nil {
		return nil, err
	}

	labelSelector := fmt.Sprintf("servicecatalog.k8s.io/spec.clusterServiceClassRef.name=%s", sc.Hash)

	servicePlans, err := client.List(ctx, metav1.ListOptions{LabelSelector: labelSelector})

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
func ListClasses(ctx context.Context, cluster *kubernetes.Cluster) (ServiceClassList, error) {
	client, err := cluster.ClientServiceCatalog("clusterserviceclasses")
	if err != nil {
		return nil, err
	}

	serviceClasses, err := client.List(ctx, metav1.ListOptions{})

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
			cluster:     cluster,
		})
	}

	return result, nil
}

// ClassLookup finds the named service class
func ClassLookup(ctx context.Context, cluster *kubernetes.Cluster, serviceClassName string) (*ServiceClass, error) {
	client, err := cluster.ClientServiceCatalog("clusterserviceclasses")
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

	serviceClasses, err := client.List(ctx, metav1.ListOptions{})

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
			cluster:     cluster,
		}, nil
	}

	// Not found
	return nil, nil
}

// CatalogServiceList returns a ServiceList of all available catalog Services
func CatalogServiceList(ctx context.Context, cluster *kubernetes.Cluster, org string) (interfaces.ServiceList, error) {
	labelSelector := fmt.Sprintf("app.kubernetes.io/name=epinio, epinio.suse.org/organization=%s", org)

	client, err := cluster.ClientServiceCatalog("serviceinstances")
	if err != nil {
		return nil, err
	}

	serviceInstances, err := client.Namespace(org).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})

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
			cluster:      cluster,
		})
	}

	return result, nil
}

// CatalogServiceLookup finds the named service
func CatalogServiceLookup(ctx context.Context, cluster *kubernetes.Cluster, org, service string) (interfaces.Service, error) {
	instanceName := serviceResourceName(org, service)

	client, err := cluster.ClientServiceCatalog("serviceinstances")
	if err != nil {
		return nil, err
	}

	serviceInstance, err := client.Namespace(org).Get(ctx, instanceName, metav1.GetOptions{})
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
		cluster:      cluster,
	}, nil
}

// CreateCatalogService creates a new catalog-based service from org,
// name, class, plan, and a string of parameter data (serialized json
// map).
func CreateCatalogService(ctx context.Context, cluster *kubernetes.Cluster, name, org, class, plan string, parameters string) (interfaces.Service, error) {
	resourceName := serviceResourceName(org, name)

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
			"clusterServicePlanExternalName": "%s",
			"parameters": %s
	  }
	}`, resourceName, org, name, org, class, plan, parameters)

	decoderUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err := decoderUnstructured.Decode([]byte(data), nil, obj)
	if err != nil {
		return nil, err
	}

	client, err := cluster.ClientServiceCatalog("serviceinstances")
	if err != nil {
		return nil, err
	}

	// todo validations - check service instance existence

	_, err = client.Namespace(org).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return &CatalogService{
		InstanceName: resourceName,
		OrgName:      org,
		Service:      name,
		Class:        class,
		Plan:         plan,
		cluster:      cluster,
	}, nil
}

// Implement the Service interface

// Name (Service interface) returns the service's name
func (s *CatalogService) Name() string {
	return s.Service
}

// Org (Service interface) returns the service's organization
func (s *CatalogService) Org() string {
	return s.OrgName
}

// GetBinding (Service interface) returns an application-specific
// secret for the service to be bound to that application.
func (s *CatalogService) GetBinding(ctx context.Context, appName string) (*corev1.Secret, error) {
	// TODO Label the secret

	bindingName := bindingResourceName(s.OrgName, s.Service, appName)

	binding, err := s.lookupBinding(ctx, bindingName, s.OrgName)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		_, err = s.createBinding(ctx, bindingName, s.OrgName, s.Service, appName)
		if err != nil {
			return nil, err
		}
	}

	return s.getBindingSecret(ctx, bindingName, s.OrgName)
}

// lookupBinding is a helper for GetBinding which finds the
// ServiceBinding object for the application with name appName, if
// there is one.
func (s *CatalogService) lookupBinding(ctx context.Context, bindingName, org string) (interface{}, error) {
	client, err := s.cluster.ClientServiceCatalog("servicebindings")
	if err != nil {
		return nil, err
	}

	serviceBinding, err := client.Namespace(org).Get(ctx, bindingName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	return serviceBinding, nil
}

// createBinding is a helper for GetBinding which creates a
// ServiceBinding for the application with name appName.
func (s *CatalogService) createBinding(ctx context.Context, bindingName, org, serviceName, appName string) (interface{}, error) {
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
	}`, bindingName, org, appName, org, serviceInstanceName, bindingName)

	decoderUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err := decoderUnstructured.Decode([]byte(data), nil, obj)
	if err != nil {
		return nil, err
	}

	client, err := s.cluster.ClientServiceCatalog("servicebindings")
	if err != nil {
		return nil, err
	}

	serviceBinding, err := client.Namespace(org).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// Update the binding secret with kubernetes app labels
	secret, err := s.getBindingSecret(ctx, bindingName, org)
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

	_, err = s.cluster.Kubectl.CoreV1().Secrets(org).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return serviceBinding, nil
}

// getBindingSecret is helper which returns the Secret that represents
// the binding of a Service to an Application.
func (s *CatalogService) getBindingSecret(ctx context.Context, bindingName, org string) (*corev1.Secret, error) {
	return s.cluster.WaitForSecret(ctx, org, bindingName, duration.ToServiceSecret())
}

// DeleteBinding (Service interface) deletes the ServiceBinding
// resource. The relevant secret will also be deleted automatically.
func (s *CatalogService) DeleteBinding(ctx context.Context, appName, org string) error {
	client, err := s.cluster.ClientServiceCatalog("servicebindings")
	if err != nil {
		return err
	}

	bindingName := bindingResourceName(s.OrgName, s.Service, appName)

	return client.Namespace(org).Delete(ctx, bindingName, metav1.DeleteOptions{})
}

// Delete (Service interface) destroys the service instance, i.e. the
// underlying kube service instance resource
func (s *CatalogService) Delete(ctx context.Context) error {
	client, err := s.cluster.ClientServiceCatalog("serviceinstances")
	if err != nil {
		return err
	}

	return client.Namespace(s.OrgName).Delete(ctx, s.InstanceName, metav1.DeleteOptions{})
}

// Status (Service interface) returns the service provision status. It
// queries the underlying service instance resource for this
func (s *CatalogService) Status(ctx context.Context) (string, error) {
	client, err := s.cluster.ClientServiceCatalog("serviceinstances")
	if err != nil {
		return "", err
	}

	serviceInstance, err := client.Namespace(s.OrgName).Get(ctx, s.InstanceName, metav1.GetOptions{})
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

// WaitForProvision (Service interface) waits for the service instance
// to be provisioned.
func (s *CatalogService) WaitForProvision(ctx context.Context) error {
	client, err := s.cluster.ClientServiceCatalog("serviceinstances")
	if err != nil {
		return err
	}

	namespace := client.Namespace(s.OrgName)

	return wait.PollImmediate(time.Second, duration.ToServiceProvision(), func() (bool, error) {
		serviceInstance, err := namespace.Get(ctx, s.InstanceName, metav1.GetOptions{})
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

// Details (Service interface) returns the service configuration,
// i.e. class and plan.
func (s *CatalogService) Details(_ context.Context) (map[string]string, error) {
	details := map[string]string{}

	details["Class"] = s.Class
	details["Plan"] = s.Plan

	return details, nil
}
