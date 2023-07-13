// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

	helmapiv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	"helm.sh/helm/v3/pkg/chartutil"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"
	"helm.sh/helm/v3/pkg/strvals"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Get returns a Service "instance" object if one is exist, or nil otherwise.
// Also returns an error if one occurs.
func (s *ServiceClient) Get(ctx context.Context, namespace, name string) (*models.Service, error) {
	var service models.Service

	serviceName := serviceResourceName(name)

	srv, err := s.kubeClient.GetSecret(ctx, namespace, serviceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// COMPATIBILITY SUPPORT - Retry for (helm controller)-based service.
			return s.GetForHelmController(ctx, namespace, name)
		}
		return nil, errors.Wrap(err, "fetching the service instance")
	}

	catalogServiceName, found := srv.GetLabels()[CatalogServiceLabelKey]
	// Secret is not labeled, act as if service is "not found"
	if !found {
		return nil, nil
	}

	catalogServiceVersion := srv.GetLabels()[CatalogServiceVersionLabelKey]

	var catalogServicePrefix string
	catalogEntry, err := s.GetCatalogService(ctx, catalogServiceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			catalogServicePrefix = "[Missing] "
		} else {
			return nil, err
		}
	}

	secretTypes := []string{}
	secretTypesAnnotationValue := srv.GetAnnotations()[CatalogServiceSecretTypesAnnotation]
	if len(secretTypesAnnotationValue) > 0 {
		secretTypes = strings.Split(secretTypesAnnotationValue, ",")
	}

	serviceInterface := s.kubeClient.Kubectl.CoreV1().Services(namespace)
	internalRoutes, err := GetInternalRoutes(ctx, serviceInterface, name)
	if err != nil {
		return nil, errors.Wrap(err, "fetching the services")
	}

	service = models.Service{
		Meta: models.Meta{
			Name:      name,
			Namespace: namespace,
			CreatedAt: srv.GetCreationTimestamp(),
		},
		SecretTypes:           secretTypes,
		CatalogService:        fmt.Sprintf("%s%s", catalogServicePrefix, catalogServiceName),
		CatalogServiceVersion: catalogServiceVersion,
		InternalRoutes:        internalRoutes,
	}

	logger := tracelog.NewLogger().WithName("ServiceStatus")

	var settings map[string]models.ChartSetting
	if catalogEntry != nil {
		settings = catalogEntry.Settings
	}

	err = setServiceStatusAndCustomValues(&service, ctx, logger, s.kubeClient,
		namespace, names.ServiceReleaseName(name), settings)

	return &service, err
}

// GetInternalRoutes returns the internal routes of the service, finding them from the kubernetes services of the Helm release
func GetInternalRoutes(ctx context.Context, servicesGetter v1.ServiceInterface, name string) ([]string, error) {
	servicesList, err := servicesGetter.List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/instance=" + names.ServiceReleaseName(name),
	})
	if err != nil {
		return nil, errors.Wrap(err, "fetching the services")
	}

	internalRoutes := []string{}
	for _, s := range servicesList.Items {
		for _, port := range s.Spec.Ports {
			route := fmt.Sprintf("%s.%s.svc.cluster.local", s.Name, s.Namespace)
			if port.Port != 80 {
				route += fmt.Sprintf(":%d", port.Port)
			}
			internalRoutes = append(internalRoutes, route)
		}
	}

	return internalRoutes, nil
}

func (s *ServiceClient) Create(ctx context.Context, namespace, name string,
	wait bool, settings models.ChartValueSettings, catalogService models.CatalogService) error {
	// Resources, and names
	//
	// |Kind	|Name		|Notes			|
	// |---		|---		|---			|
	// |secret	|"s-"+name	|epinio management data	|
	// |helm release|see above	|active workload	|

	service := serviceResourceName(name)
	labels := map[string]string{
		CatalogServiceLabelKey:        catalogService.Meta.Name,
		CatalogServiceVersionLabelKey: catalogService.AppVersion,
		ServiceNameLabelKey:           name,
	}

	var annotations map[string]string // default: nil
	if len(catalogService.SecretTypes) > 0 {
		annotations = map[string]string{
			CatalogServiceSecretTypesAnnotation: strings.Join(catalogService.SecretTypes, ","),
		}
	}

	err := s.kubeClient.CreateLabeledSecret(ctx, namespace, service, nil, labels, annotations)
	if err != nil {
		return errors.Wrap(err, "error creating service secret")
	}

	epinioValues, err := getEpinioValues(name, catalogService.Meta.Name)
	if err != nil {
		logger := tracelog.NewLogger().WithName("Create")
		logger.Error(err, "getting epinio values")
	}

	// Ingest the service class YAML data into a proper values table
	classValues, err := chartutil.ReadValues([]byte(catalogService.Values + epinioValues))
	if err != nil {
		return errors.Wrap(err, "failed to read service class values")
	}

	// Create proper values table from the --chart-value option data
	userValues := chartutil.Values{}
	for key, value := range settings {
		err := strvals.ParseInto(key+"="+value, userValues)
		if err != nil {
			return errors.Wrap(err, "failed to parse `"+key+"="+value+"`")
		}
	}

	// Merge class and user values, then serialize back to YAML.
	//
	// NOTE: Class values have priority over user values, under the assumption that these are
	// needed to (1) have the service chart working with Epinio (*), or (2) are chosen by the
	// operator for their environment.
	//
	// (*) See the `extraDeploy` setting found in dev service classes.
	//
	// ATTENTION: This priority order is reversed from what is said in the application CRD PR.
	// FIX:       application CRD PR to match here.

	values, err := chartutil.Values(chartutil.CoalesceTables(classValues, userValues)).YAML()
	if err != nil {
		return errors.Wrap(err, "failed to merge class and user values")
	}

	err = helm.DeployService(
		ctx,
		helm.ServiceParameters{
			AppRef:     models.NewAppRef(name, namespace),
			Cluster:    s.kubeClient,
			Chart:      catalogService.HelmChart,
			Version:    catalogService.ChartVersion,
			Repository: catalogService.HelmRepo.URL,
			Values:     values,
			Wait:       wait,
		})

	if err != nil {
		errb := s.kubeClient.DeleteSecret(ctx, namespace, service)
		if errb != nil {
			return errors.Wrap(errb, "error deploying service helm chart while undoing the secret")
		}
	}

	return errors.Wrap(err, "error deploying service helm chart")
}

// Delete deletes the helmcharts that matches the given service which is installed on the namespace
func (s *ServiceClient) Delete(ctx context.Context, namespace, name string) error {
	service := serviceResourceName(name)

	err := helm.RemoveService(
		requestctx.Logger(ctx),
		s.kubeClient,
		models.NewAppRef(name, namespace),
	)
	if err != nil {
		return errors.Wrap(err, "error deleting service helm release")
	}

	err = s.kubeClient.DeleteSecret(ctx, namespace, service)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// COMPATIBILITY SUPPORT - Retry for (helm controller)-based service
			return s.DeleteForHelmController(ctx, namespace, name)
		}
		return errors.Wrap(err, "error deleting service secret")
	}

	return nil
}

// DeleteAll deletes all helmcharts installed on the specified namespace.
// It's used to cleanup before a namespace is deleted.
func (s *ServiceClient) DeleteAll(ctx context.Context, namespace string) error {
	services, err := s.kubeClient.Kubectl.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"%s,%s",
			ServiceNameLabelKey,
			CatalogServiceLabelKey,
		),
	})

	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrap(err, "listing the service instances")
	}

	for _, srv := range services.Items {
		// Inlined Delete() ... Avoids back and forth conversion between service and secret names
		err := s.kubeClient.DeleteSecret(ctx, srv.ObjectMeta.Namespace, srv.ObjectMeta.Name)
		if err != nil {
			return errors.Wrap(err, "error deleting service secret")
		}

		service := srv.GetLabels()[ServiceNameLabelKey]

		err = helm.RemoveService(requestctx.Logger(ctx),
			s.kubeClient,
			models.NewAppRef(service, srv.ObjectMeta.Namespace))
		if err != nil {
			return errors.Wrap(err, "error deleting service helm release")
		}
	}

	// COMPATIBILITY SUPPORT - Remove all (helm controller)-based services too.
	return s.DeleteAllForHelmController(ctx, namespace)
}

// ListAll will return all the Epinio Service instances
func (s *ServiceClient) ListAll(ctx context.Context) (models.ServiceList, error) {
	return s.list(ctx, "")
}

// ListInNamespace will return all the Epinio Services available in the specified namespace
func (s *ServiceClient) ListInNamespace(ctx context.Context, namespace string) (models.ServiceList, error) {
	return s.list(ctx, namespace)
}

// list will return all the Epinio Services available in the targeted namespace.
// If the namespace is blank it will return all the instances from all the namespaces
func (s *ServiceClient) list(ctx context.Context, namespace string) (models.ServiceList, error) {
	serviceList := models.ServiceList{}

	listOpts := metav1.ListOptions{}
	listOpts.LabelSelector = fmt.Sprintf(
		"%s,%s",
		ServiceNameLabelKey,
		CatalogServiceLabelKey,
	)

	services, err := s.kubeClient.Kubectl.CoreV1().Secrets(namespace).List(ctx, listOpts)

	if err != nil {
		if apierrors.IsNotFound(err) {
			return serviceList, nil
		}
		return nil, errors.Wrap(err, "listing the service instances")
	}

	catalogServices, err := s.ListCatalogServices(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting catalog services")
	}

	// catalogServiceNameMap is a lookup map to check the available Catalog Services
	catalogServiceNameMap := map[string]struct{}{}
	for _, catalogService := range catalogServices {
		catalogServiceNameMap[catalogService.Meta.Name] = struct{}{}
	}

	for _, srv := range services.Items {
		catalogServiceName := srv.GetLabels()[CatalogServiceLabelKey]
		if _, exists := catalogServiceNameMap[catalogServiceName]; !exists {
			catalogServiceName = "[Missing] " + catalogServiceName
		}

		serviceName := srv.GetLabels()[ServiceNameLabelKey]

		service := models.Service{
			Meta: models.Meta{
				Name:      serviceName,
				Namespace: srv.ObjectMeta.Namespace,
				CreatedAt: srv.GetCreationTimestamp(),
			},
			CatalogService:        catalogServiceName,
			CatalogServiceVersion: srv.GetLabels()[CatalogServiceVersionLabelKey],
		}

		logger := tracelog.NewLogger().WithName("ServiceStatus")

		err = setServiceStatusAndCustomValues(&service, ctx, logger, s.kubeClient,
			srv.ObjectMeta.Namespace, names.ServiceReleaseName(serviceName),
			nil, // no settings information - TODO
		)
		if err != nil {
			return nil, err
		}

		serviceList = append(serviceList, service)
	}

	// COMPATIBILITY SUPPORT - List (helm controller)-based services too.
	serviceListHC, err := s.listForHelmController(ctx, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "listing the (helm controller)-based service instances")
	}

	return append(serviceList, serviceListHC...), nil
}

func serviceResourceName(name string) string {
	return names.GenerateResourceName("s", name)
}

// -----------------------------------------------------------------------------------------------
// COMPATIBILITY SUPPORT for services from before https://github.com/epinio/epinio/issues/1704 fix
//
// This is essentially all of the old Get/Delete(All)/List* functions, renamed with an added
// `HelmController` suffix. The new functions run them in appropriate places.
//
// NOTE that `Create` is NOT in this list. We do not create (helm controller)-based services anymore.
//

// GetForHelmController returns a Service "instance" object if one is exist, or nil otherwise.  Also
// returns an error if one occurs.
func (s *ServiceClient) GetForHelmController(ctx context.Context, namespace, name string) (*models.Service, error) {
	var service models.Service

	helmChartName := names.ServiceHelmChartName(name, namespace)
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

	catalogServiceVersion := srv.GetLabels()[CatalogServiceVersionLabelKey]

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

	secretTypes := []string{}
	secretTypesAnnotationValue := srv.GetAnnotations()[CatalogServiceSecretTypesAnnotation]
	if len(secretTypesAnnotationValue) > 0 {
		secretTypes = strings.Split(secretTypesAnnotationValue, ",")
	}

	service = models.Service{
		Meta: models.Meta{
			Name:      name,
			Namespace: targetNamespace,
			CreatedAt: srv.GetCreationTimestamp(),
		},
		SecretTypes:             secretTypes,
		CatalogService:          fmt.Sprintf("%s%s", catalogServicePrefix, catalogServiceName),
		CatalogServiceVersion:   catalogServiceVersion,
		ManagedByHelmController: true,
	}

	logger := tracelog.NewLogger().WithName("ServiceStatus")

	err = setServiceStatusAndCustomValues(&service, ctx, logger, s.kubeClient,
		targetNamespace, helmChartName,
		nil, // no service settings -- HC -- Will be removed anyway
	)
	if err != nil {
		return nil, err
	}

	return &service, nil
}

// DeleteForHelmController deletes the helmcharts that matches the given service which is installed
// on the namespace (that's the targetNamespace).
func (s *ServiceClient) DeleteForHelmController(ctx context.Context, namespace, service string) error {
	err := s.helmChartsKubeClient.Namespace(helmchart.Namespace()).Delete(ctx,
		names.ServiceHelmChartName(service, namespace),
		metav1.DeleteOptions{},
	)

	if apierrors.IsNotFound(err) {
		return nil
	}

	return errors.Wrap(err, "error deleting helm chart @"+namespace+"/"+service)
}

// DeleteAllForHelmController deletes all helmcharts installed on the specified namespace.  It is
// used to cleanup before a namespace is deleted.  The targetNamespace is not the namespace where
// the helmchart resource resides (that would be `epinio`) but the `targetNamespace` field of the
// helmchart.
func (s *ServiceClient) DeleteAllForHelmController(ctx context.Context, targetNamespace string) error {
	err := s.helmChartsKubeClient.Namespace(helmchart.Namespace()).DeleteCollection(ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", TargetNamespaceLabelKey, targetNamespace),
		},
	)

	if apierrors.IsNotFound(err) {
		return nil
	}

	return errors.Wrap(err, "error deleting helm charts in "+targetNamespace)
}

// listForHelmController will return all the Epinio Services available in the targeted namespace.
// If the namespace is blank it will return all the instances from all the namespaces
func (s *ServiceClient) listForHelmController(ctx context.Context, namespace string) (models.ServiceList, error) {
	serviceList := models.ServiceList{}

	listOpts := metav1.ListOptions{}
	if namespace == "" {
		listOpts.LabelSelector = fmt.Sprintf("%s,%s", ServiceNameLabelKey, CatalogServiceLabelKey)
	} else {
		listOpts.LabelSelector = fmt.Sprintf(
			"%s,%s,%s=%s",
			ServiceNameLabelKey,
			CatalogServiceLabelKey,
			TargetNamespaceLabelKey, namespace,
		)
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
		catalogServiceNameMap[catalogService.Meta.Name] = struct{}{}
	}

	for _, srv := range helmChartList {

		catalogServiceName := srv.GetLabels()[CatalogServiceLabelKey]
		if _, exists := catalogServiceNameMap[catalogServiceName]; !exists {
			catalogServiceName = "[Missing] " + catalogServiceName
		}

		service := models.Service{
			Meta: models.Meta{
				Name:      srv.GetLabels()[ServiceNameLabelKey],
				Namespace: srv.GetLabels()[TargetNamespaceLabelKey],
				CreatedAt: srv.GetCreationTimestamp(),
			},
			CatalogService:          catalogServiceName,
			CatalogServiceVersion:   srv.GetLabels()[CatalogServiceVersionLabelKey],
			ManagedByHelmController: true,
		}

		logger := tracelog.NewLogger().WithName("ServiceStatus")

		err = setServiceStatusAndCustomValues(&service, ctx, logger, s.kubeClient, srv.Spec.TargetNamespace, srv.Name, nil /* no service settings - list, and HC, to be removed */)
		if err != nil {
			return nil, err
		}

		serviceList = append(serviceList, service)
	}

	return serviceList, nil
}

func NewServiceStatusFromHelmRelease(status helm.ReleaseStatus) models.ServiceStatus {
	switch status {
	case helm.StatusReady:
		return models.ServiceStatusDeployed
	case helm.StatusNotReady:
		return models.ServiceStatusNotReady
	default:
		return models.ServiceStatusUnknown
	}
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

func getEpinioValues(serviceName, catalogServiceName string) (string, error) {
	//epinio:
	//  serviceName: serviceName
	//  catalogServiceName: catalogServiceName

	type epinioValues struct {
		ServiceName        string `yaml:"serviceName,omitempty"`
		CatalogServiceName string `yaml:"catalogServiceName,omitempty"`
	}

	extraValues := epinioValues{
		ServiceName:        serviceName,
		CatalogServiceName: catalogServiceName,
	}

	yamlData, err := yaml.Marshal(&struct {
		Epinio epinioValues `yaml:"epinio,omitempty"`
	}{extraValues})
	if err != nil {
		return "", err
	}

	return "\n" + string(yamlData), nil
}

func setServiceStatusAndCustomValues(service *models.Service,
	ctx context.Context, logger logr.Logger, cluster *kubernetes.Cluster,
	namespace, releaseName string,
	settings map[string]models.ChartSetting,
) error {
	serviceRelease, err := helm.Release(ctx, logger, cluster, namespace, releaseName)

	if err != nil {
		if errors.Is(err, helmdriver.ErrReleaseNotFound) {
			service.Status = "Not Ready" // The installation job is still running?
			return nil
		}
		return errors.Wrap(err, "finding helm release status")
	}

	serviceStatus, err := helm.Status(ctx, logger, cluster, namespace, releaseName,
		serviceRelease)
	if err != nil {
		return errors.Wrap(err, "calculating helm release status")
	}

	service.Status = NewServiceStatusFromHelmRelease(serviceStatus)

	if len(settings) > 0 {
		customized := models.ChartValueSettings{}
		configValues := chartutil.Values(serviceRelease.Config)

		for key := range settings {
			customValue, err := configValues.PathValue(key)
			if err != nil {
				// Not found - This custom value was not customized by the user.
				// That is ok. Nothing to report.
				continue
			}
			customValueAsString := fmt.Sprintf("%v", customValue)
			customized[key] = customValueAsString
		}

		if len(customized) > 0 {
			service.Settings = customized
		}
	}
	return nil
}
