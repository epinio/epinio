package services

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	helmapiv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Get returns a Service "instance" object if one is exist, or nil otherwise.
// Also returns an error if one occurs.
func (s *ServiceClient) Get(ctx context.Context, namespace, name string) (*models.Service, error) {
	var service models.Service

	helmChartName := models.ServiceHelmChartName(name, namespace)
	srv, err := s.helmChartsKubeClient.Namespace(helmchart.Namespace()).Get(ctx, helmChartName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "fetching the service instance")
	}

	catalogServiceName, found := srv.GetLabels()[CatalogServiceLabelKey]

	// Helmchart is not labeled, act as if service is "not found"
	if !found {
		return nil, nil
	}

	var catalogServicePrefix string
	_, err = s.GetCatalogService(ctx, catalogServiceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			catalogServicePrefix = "[Missing] "
		} else {
			return nil, err
		}
	}

	targetNamespace, found, err := unstructured.NestedString(srv.UnstructuredContent(), "spec", "targetNamespace")
	if err != nil {
		return nil, errors.Wrapf(err, "looking up targetNamespace as a string")
	}
	if !found {
		return nil, errors.New("targetNamespace field not found")
	}

	service = models.Service{
		Name:           name,
		Namespace:      targetNamespace,
		CatalogService: fmt.Sprintf("%s%s", catalogServicePrefix, catalogServiceName),
	}

	logger := tracelog.NewLogger().WithName("ServiceStatus")
	serviceStatus, err := helm.Status(ctx, logger, s.kubeClient, targetNamespace, helmChartName)
	if err != nil {
		if errors.Is(err, helmdriver.ErrReleaseNotFound) {
			serviceStatus = "Not Ready" // The installation job is still running?
		} else {
			return &service, errors.Wrap(err, "finding helm release status")
		}
	}

	service.Status = models.NewServiceStatusFromHelmRelease(serviceStatus)

	return &service, nil
}

func (s *ServiceClient) Create(ctx context.Context, namespace, name string, catalogService models.CatalogService) error {
	helmChart := &helmapiv1.HelmChart{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "helm.cattle.io/v1",
			Kind:       "HelmChart",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      models.ServiceHelmChartName(name, namespace),
			Namespace: helmchart.Namespace(),
			Labels: map[string]string{
				CatalogServiceLabelKey:  catalogService.Name,
				TargetNamespaceLabelKey: namespace,
				ServiceNameLabelKey:     name,
			},
		},
		Spec: helmapiv1.HelmChartSpec{
			TargetNamespace: namespace,
			Chart:           catalogService.HelmChart,
			Repo:            catalogService.HelmRepo.URL,
			ValuesContent:   catalogService.Values,
		},
	}

	mapHelmChart, err := runtime.DefaultUnstructuredConverter.ToUnstructured(helmChart)
	if err != nil {
		return errors.Wrap(err, "error converting helmChart to unstructured")
	}

	unstructureHelmChart := &unstructured.Unstructured{}
	unstructureHelmChart.SetUnstructuredContent(mapHelmChart)

	_, err = s.helmChartsKubeClient.Namespace(helmchart.Namespace()).Create(
		ctx, unstructureHelmChart, metav1.CreateOptions{})
	return errors.Wrap(err, "error creating helm chart")
}

// Delete deletes the helmcharts that matches the given service which is
// installed on the namespace (that's the targetNamespace).
func (s *ServiceClient) Delete(ctx context.Context, namespace, service string) error {
	err := s.helmChartsKubeClient.Namespace(helmchart.Namespace()).Delete(ctx,
		models.ServiceHelmChartName(service, namespace),
		metav1.DeleteOptions{},
	)

	return errors.Wrap(err, "error deleting helm charts")
}

// DeleteAll deletes all helmcharts installed on the specified namespace.
// It's used to cleanup before a namespace is deleted.
// The targetNamespace is not the namespace where the helmchart resource resides
// (that would be `epinio`) but the `targetNamespace` field of the helmchart.
func (s *ServiceClient) DeleteAll(ctx context.Context, targetNamespace string) error {
	err := s.helmChartsKubeClient.Namespace(helmchart.Namespace()).DeleteCollection(ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", TargetNamespaceLabelKey, targetNamespace),
		},
	)

	return errors.Wrap(err, "error deleting helm charts")
}

// List will return all the Epinio Services available in the targeted namespace
func (s *ServiceClient) List(ctx context.Context, namespace string) ([]*models.Service, error) {
	serviceList := []*models.Service{}

	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"%s,%s,%s=%s",
			ServiceNameLabelKey,
			CatalogServiceLabelKey,
			TargetNamespaceLabelKey, namespace,
		),
	}

	unstructuredServiceList, err := s.helmChartsKubeClient.Namespace(helmchart.Namespace()).List(ctx, listOpts)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return serviceList, nil
		}
		return nil, errors.Wrap(err, "fetching the service instance")
	}

	helmChartList, err := convertUnstructuredListIntoHelmCharts(unstructuredServiceList)
	if err != nil {
		return nil, errors.Wrap(err, "error converting unstructured list to helm charts")
	}

	catalogServices, err := s.ListCatalogServices(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting catalog services")
	}

	// catalogServiceNameMap is a lookup map to check the available Catalog Services
	catalogServiceNameMap := map[string]struct{}{}
	for _, catalogService := range catalogServices {
		catalogServiceNameMap[catalogService.Name] = struct{}{}
	}

	for _, srv := range helmChartList {

		catalogServiceName := srv.GetLabels()[CatalogServiceLabelKey]
		if _, exists := catalogServiceNameMap[catalogServiceName]; !exists {
			catalogServiceName = "[Missing] " + catalogServiceName
		}

		service := models.Service{
			Name:           srv.GetLabels()[ServiceNameLabelKey],
			Namespace:      namespace,
			CatalogService: catalogServiceName,
		}

		logger := tracelog.NewLogger().WithName("ServiceStatus")
		serviceStatus, err := helm.Status(ctx, logger, s.kubeClient, srv.Spec.TargetNamespace, srv.Name)
		if err != nil {
			if errors.Is(err, helmdriver.ErrReleaseNotFound) {
				serviceStatus = "Not Ready" // The installation job is still running?
			} else {
				return nil, errors.Wrap(err, "finding helm release status")
			}
		}

		service.Status = models.NewServiceStatusFromHelmRelease(serviceStatus)

		serviceList = append(serviceList, &service)
	}

	return serviceList, nil
}

func convertUnstructuredListIntoHelmCharts(unstructuredList *unstructured.UnstructuredList) ([]helmapiv1.HelmChart, error) {
	helmChartList := []helmapiv1.HelmChart{}

	for _, srv := range unstructuredList.Items {
		helmChart := helmapiv1.HelmChart{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(srv.Object, &helmChart)
		if err != nil {
			return nil, errors.Wrap(err, "error converting helmchart")
		}

		helmChartList = append(helmChartList, helmChart)
	}

	return helmChartList, nil
}
