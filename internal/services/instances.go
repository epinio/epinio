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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"

	"helm.sh/helm/v3/pkg/chartutil"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"
	"helm.sh/helm/v3/pkg/strvals"
	corev1 "k8s.io/api/core/v1"
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
			return nil, nil
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

	err = setServiceStatusAndCustomValues(&service, srv, ctx, logger, s.kubeClient,
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

func (s *ServiceClient) Create(ctx context.Context,
	namespace, name string,
	wait bool,
	settings models.ChartValueSettings,
	catalogService *models.CatalogService,
	hook helm.PostDeployFunction,
) error {
	// Resources, and names
	//
	// |Kind	|Name		|Notes			|
	// |---		|---		|---			|
	// |secret	|"s-"+name	|epinio management data	|
	// |helm release|see above	|active workload	|

	// Create the secret first

	service := serviceResourceName(name)
	labels := map[string]string{
		CatalogServiceLabelKey:        catalogService.Meta.Name,
		CatalogServiceVersionLabelKey: catalogService.AppVersion,
		ServiceNameLabelKey:           name,
	}

	var data map[string][]byte
	if settings != nil {
		yaml, err := yaml.Marshal(settings)
		if err != nil {
			return errors.Wrap(err, "failed to marshall the settings")
		}
		data = map[string][]byte{
			"settings": yaml,
		}
	}

	var annotations map[string]string // default: nil
	if len(catalogService.SecretTypes) > 0 {
		annotations = map[string]string{
			CatalogServiceSecretTypesAnnotation: strings.Join(catalogService.SecretTypes, ","),
		}
	}

	err := s.kubeClient.CreateLabeledSecret(ctx, namespace, service, data, labels, annotations)
	if err != nil {
		return errors.Wrap(err, "failed to create service secret")
	}

	// The secret representing the service is created. Now deploy the helm chart.

	err = s.DeployOrUpdate(ctx, namespace, name, wait, settings, catalogService, hook)
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
		// [NF] NOTE: The err is some nested thing with a `not found` at the bottom.  The
		// `apierrors.IsNotFound` does not recognize that. Its docs claim that it searches
		// through the tree of wrapped errors. Maybe the `not found` is not a kube not
		// found -- Helm ?
		// ===> For now performing check by string match.
		if !strings.Contains(err.Error(), "not found") {
			return errors.Wrap(err, "error deleting service helm release")
		}
		// not found -> NAME may be a partially created service, i.e. secret exists, helm release does not.
		// -> continue to deletion of the secret.
	}

	err = s.kubeClient.DeleteSecret(ctx, namespace, service)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
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
		service := srv.GetLabels()[ServiceNameLabelKey]

		err = helm.RemoveService(requestctx.Logger(ctx),
			s.kubeClient,
			models.NewAppRef(service, srv.ObjectMeta.Namespace))
		if err != nil {
			// See [NF] for details
			if !strings.Contains(err.Error(), "not found") {
				return errors.Wrap(err, "error deleting service helm release")
			}
		}
		err := s.kubeClient.DeleteSecret(ctx, srv.ObjectMeta.Namespace, srv.ObjectMeta.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return errors.Wrap(err, "error deleting service secret")
		}
	}

	return nil
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

		theServiceSecret := srv
		err = setServiceStatusAndCustomValues(&service, &theServiceSecret, ctx, logger, s.kubeClient,
			srv.ObjectMeta.Namespace, names.ServiceReleaseName(serviceName),
			nil, // no settings information - TODO
		)
		if err != nil {
			return nil, err
		}

		serviceList = append(serviceList, service)
	}

	return serviceList, nil
}

func serviceResourceName(name string) string {
	return names.GenerateResourceName("s", name)
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

// UpdateService modifies an existing service as per the instructions and writes
// the result back to the resource.
func (s *ServiceClient) UpdateService(ctx context.Context, cluster *kubernetes.Cluster, service *models.Service,
	changes models.ServiceUpdateRequest, hook helm.PostDeployFunction) error {

	// Update the secret first. As part of that we get the updated settings as well.

	var newSettings models.ChartValueSettings
	serviceSecretName := serviceResourceName(service.Meta.Name)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		serviceSecret, err := cluster.GetSecret(ctx, service.Meta.Namespace, serviceSecretName)
		if err != nil {
			return err
		}

		// ATTENTION: This code does not fall back to heuristic extraction from the helm
		// release of the service as `Get` does.

		// Read existing settings
		settings := models.ChartValueSettings{}

		if serviceSecret.Data != nil {
			yamlSettings, ok := serviceSecret.Data["settings"]
			if ok {
				// Found the exact settings in the K secret representing the E service
				err := yaml.Unmarshal(yamlSettings, &settings)
				if err != nil {
					return errors.Wrap(err, "failed to unmarshall the settings")
				}
			}
		}

		// Modify the settings as per the instructions.
		for _, remove := range changes.Remove {
			delete(settings, remove)
		}
		for key, value := range changes.Set {
			settings[key] = value
		}

		yaml, err := yaml.Marshal(settings)
		if err != nil {
			return errors.Wrap(err, "failed to marshall the settings")
		}

		// Write the settings back
		if serviceSecret.Data == nil {
			serviceSecret.Data = map[string][]byte{}
		}

		serviceSecret.Data["settings"] = yaml

		_, err = cluster.Kubectl.CoreV1().Secrets(service.Namespace()).Update(
			ctx, serviceSecret, metav1.UpdateOptions{})
		if err != nil {
			// publish to calling scope
			newSettings = settings
		}

		return err
	})

	if err != nil {
		return err
	}

	catalogService, err := s.GetCatalogService(ctx, service.CatalogService)
	if err != nil {
		return err
	}

	err = s.DeployOrUpdate(ctx, service.Meta.Namespace, service.Meta.Name, changes.Wait,
		newSettings, catalogService, hook)

	return errors.Wrap(err, "error deploying service helm chart")

}

// ReplaceService replaces an existing service
func (s *ServiceClient) ReplaceService(ctx context.Context, cluster *kubernetes.Cluster, service *models.Service,
	data models.ServiceReplaceRequest, hook helm.PostDeployFunction) (bool, error) {
	changed := false

	var newSettings models.ChartValueSettings
	serviceSecretName := serviceResourceName(service.Meta.Name)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		serviceSecret, err := cluster.GetSecret(ctx, service.Meta.Namespace, serviceSecretName)
		if err != nil {
			return err
		}

		// Read existing settings
		oldSettings := models.ChartValueSettings{}

		if serviceSecret.Data != nil {
			yamlSettings, ok := serviceSecret.Data["settings"]
			if ok {
				// Found the exact settings in the K secret representing the E service
				err := yaml.Unmarshal(yamlSettings, &oldSettings)
				if err != nil {
					return errors.Wrap(err, "failed to unmarshall the settings")
				}
			}
		}

		// Bail out without writing when there is no actual change.
		if reflect.DeepEqual(oldSettings, data.Settings) {
			// still `changed == false`
			return nil
		}

		// Write new data
		yaml, err := yaml.Marshal(data.Settings)
		if err != nil {
			return errors.Wrap(err, "failed to marshall the settings")
		}

		if serviceSecret.Data == nil {
			serviceSecret.Data = map[string][]byte{}
		}

		serviceSecret.Data["settings"] = yaml

		_, err = cluster.Kubectl.CoreV1().Secrets(service.Namespace()).Update(
			ctx, serviceSecret, metav1.UpdateOptions{})

		if err == nil {
			changed = true
			// publish new state to calling scope
			newSettings = data.Settings
		}

		return err
	})
	if err != nil {
		return false, err
	}

	if changed {
		catalogService, err := s.GetCatalogService(ctx, service.CatalogService)
		if err != nil {
			return false, err
		}

		// push new state to helm release
		err = s.DeployOrUpdate(ctx, service.Meta.Namespace, service.Meta.Name, data.Wait,
			newSettings, catalogService, hook)
		if err != nil {
			return false, err
		}
	}

	return changed, nil
}

// Deploy deploys the helm chart of a service, or updates its release.
func (s *ServiceClient) DeployOrUpdate(
	ctx context.Context,
	namespace, name string,
	wait bool,
	settings models.ChartValueSettings,
	catalogService *models.CatalogService,
	hook helm.PostDeployFunction) error {

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

	return helm.DeployService(ctx,
		helm.ServiceParameters{
			AppRef:         models.NewAppRef(name, namespace),
			Cluster:        s.kubeClient,
			CatalogService: *catalogService,
			Values:         values,
			Wait:           wait,
			PostDeployHook: hook,
		})
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
	serviceSecret *corev1.Secret,
	ctx context.Context, logger logr.Logger, cluster *kubernetes.Cluster,
	namespace, releaseName string,
	settings map[string]models.ChartSetting,
) error {
	serviceRelease, err := helm.Release(ctx, logger, cluster, namespace, releaseName)

	if err != nil {
		if errors.Is(err, helmdriver.ErrReleaseNotFound) {
			service.Status = models.ServiceStatusNotReady // The installation job is still running?
			return nil
		}
		return errors.Wrap(err, "finding helm release status")
	}

	service.Status = models.ServiceStatusUnknown

	serviceStatus, err := helm.Status(ctx, logger, cluster, serviceRelease)
	if err != nil {
		return errors.Wrap(err, "calculating helm release status")
	}

	service.Status = NewServiceStatusFromHelmRelease(serviceStatus)

	yamlSettings, ok := serviceSecret.Data["settings"]
	if ok {
		// Found the exact settings in the K secret representing the E service
		var customized models.ChartValueSettings
		err := yaml.Unmarshal(yamlSettings, &customized)
		if err != nil {
			return errors.Wrap(err, "failed to unmarshall the settings")
		}

		service.Settings = customized
		return nil
	}

	// The settings are not in the K secret. This is a 1.9.0-created service. Fall back to
	// heuristic (i.e. hacked) extraction of the settings from the Helm release for backward
	// compatibility. This is not a 1:1 roundtrip

	if len(settings) > 0 {
		customized := models.ChartValueSettings{}
		configValues := chartutil.Values(serviceRelease.Config)

		for key := range settings {
			value, err := getValue(configValues, key, true)
			if err == nil {
				customized[key] = value
				continue
			}

			// Not found as-is. Walk the path up to see if there is a match for a prefix
			// of the setting. This is possible if there is an inner array keeping
			// things from being a pure nested map.

			pieces := strings.Split(key, ".")
			pieces = pieces[0 : len(pieces)-1]

			for len(pieces) > 0 {
				key := strings.Join(pieces, ".")

				value, err := getValue(configValues, key, false)
				if err == nil {
					customized[key] = value
					break
				}

				pieces = pieces[0 : len(pieces)-1]
			}
		}

		if len(customized) > 0 {
			service.Settings = customized
		}
	}
	return nil
}

func getValue(values chartutil.Values, key string, maybetable bool) (string, error) {
	value, err := values.PathValue(key)
	if err == nil {
		data, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		valueAsString := string(data)
		return valueAsString, nil
	}
	if maybetable {
		tvalue, terr := values.Table(key)
		if terr == nil {
			data, err := json.Marshal(tvalue)
			if err != nil {
				return "", err
			}
			valueAsString := string(data)
			return valueAsString, nil
		}
	}
	return "", err
}
