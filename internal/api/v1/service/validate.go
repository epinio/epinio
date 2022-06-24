package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/services"
	"github.com/go-logr/logr"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	helmrelease "helm.sh/helm/v3/pkg/release"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"
)

// ValidateService is used by various service endpoints to verify that the service exists,
// as well as its helm release, before action is taken.
func ValidateService(
	ctx context.Context, cluster *kubernetes.Cluster, logger logr.Logger,
	namespace, service string,
) apierror.APIErrors {

	logger.Info("get service client")

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	logger.Info("get service")

	theService, err := kubeServiceClient.Get(ctx, namespace, service)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return apierror.NewNotFoundError("service not found")
		}

		return apierror.NewInternalError(err)
	}
	// See internal/services/instances.go - not found => no error, nil structure
	if theService == nil {
		return apierror.NewNotFoundError("service not found")
	}

	logger.Info("getting helm client")

	client, err := helm.GetHelmClient(cluster.RestConfig, logger, namespace)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	logger.Info("looking for service release")

	releaseName := names.ServiceHelmChartName(service, namespace)
	srv, err := client.GetRelease(releaseName)
	if err != nil {
		if errors.Is(err, helmdriver.ErrReleaseNotFound) {
			return apierror.NewNotFoundError(fmt.Sprintf("%s - %s", err.Error(), releaseName))
		}
		return apierror.NewInternalError(err)
	}

	logger.Info(fmt.Sprintf("service found %+v\n", service))
	if srv.Info.Status != helmrelease.StatusDeployed {
		return apierror.NewInternalError(err)
	}

	return nil
}
