package service

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/services"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	helmrelease "helm.sh/helm/v3/pkg/release"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"
)

// FindAndValidateService is used by various service endpoints to verify that the service exists,
// as well as its helm release, before action is taken.
// It will find the service with the provided namespace and name
func FindAndValidateService(
	ctx context.Context, cluster *kubernetes.Cluster, logger logr.Logger,
	namespace, serviceName string,
) apierror.APIErrors {
	service, apiErr := GetService(ctx, cluster, logger, namespace, serviceName)
	if apiErr != nil {
		return apiErr
	}

	return ValidateService(ctx, cluster, logger, service)
}

// GetService will find the service with the provided namespace and name
func GetService(
	ctx context.Context, cluster *kubernetes.Cluster, logger logr.Logger,
	namespace, serviceName string,
) (*models.Service, apierror.APIErrors) {

	logger.Info("get service client")

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return nil, apierror.InternalError(err)
	}

	logger.Info("get service")

	theService, err := kubeServiceClient.Get(ctx, namespace, serviceName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, apierror.NewNotFoundError("service", serviceName).WithDetails(err.Error())
		}

		return nil, apierror.InternalError(err)
	}
	// See internal/services/instances.go - not found => no error, nil structure
	if theService == nil {
		return nil, apierror.NewNotFoundError("service", serviceName)
	}

	logger.Info(fmt.Sprintf("service found %+v\n", serviceName))
	return theService, nil
}

// ValidateService is used by various service endpoints to verify that the service exists,
// as well as its helm release, before action is taken.
func ValidateService(
	ctx context.Context, cluster *kubernetes.Cluster, logger logr.Logger,
	service *models.Service,
) apierror.APIErrors {

	logger.Info("getting helm client")

	client, err := helm.GetHelmClient(cluster.RestConfig, logger, service.Namespace())
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("looking for service release")

	releaseName := names.ServiceHelmChartName(service.Meta.Name, service.Namespace())
	srv, err := client.GetRelease(releaseName)
	if err != nil {
		if errors.Is(err, helmdriver.ErrReleaseNotFound) {
			return apierror.NewNotFoundError("release", releaseName).WithDetailsf(err.Error())
		}
		return apierror.InternalError(err)
	}

	if srv.Info.Status != helmrelease.StatusDeployed {
		return apierror.InternalError(err)
	}

	return nil
}
