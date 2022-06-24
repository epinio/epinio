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
		return apierror.InternalError(err)
	}

	logger.Info("get service")

	theService, err := kubeServiceClient.Get(ctx, namespace, service)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return apierror.NewNotFoundError(errors.Wrap(err, "service not found"))
		}

		return apierror.InternalError(err)
	}
	// See internal/services/instances.go - not found => no error, nil structure
	if theService == nil {
		return apierror.NewNotFoundError(errors.New("service not found"))
	}

	logger.Info("getting helm client")

	client, err := helm.GetHelmClient(cluster.RestConfig, logger, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("looking for service release")

	releaseName := names.ServiceHelmChartName(service, namespace)
	srv, err := client.GetRelease(releaseName)
	if err != nil {
		if errors.Is(err, helmdriver.ErrReleaseNotFound) {
			return apierror.NewNotFoundError(errors.Wrapf(err, "release %s not found", releaseName))
		}
		return apierror.InternalError(err)
	}

	logger.Info(fmt.Sprintf("service found %+v\n", service))
	if srv.Info.Status != helmrelease.StatusDeployed {
		return apierror.InternalError(err)
	}

	return nil
}
