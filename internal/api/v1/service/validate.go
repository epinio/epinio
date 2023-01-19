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

	releaseName := names.ServiceReleaseName(service.Meta.Name)
	srv, err := client.GetRelease(releaseName)
	if err != nil {
		if !errors.Is(err, helmdriver.ErrReleaseNotFound) {
			return apierror.InternalError(err)
		}

		// COMPATIBILITY SUPPORT for services from before https://github.com/epinio/epinio/issues/1704 fix
		releaseNameHC := names.ServiceHelmChartName(service.Meta.Name, service.Namespace())
		srv, err = client.GetRelease(releaseNameHC)
		if err != nil {
			if errors.Is(err, helmdriver.ErrReleaseNotFound) {
				return apierror.NewNotFoundError("release", releaseName).WithDetailsf(err.Error())
			}
			return apierror.InternalError(err)
		}
	}

	if srv.Info.Status != helmrelease.StatusDeployed {
		return apierror.InternalError(err)
	}

	return nil
}
