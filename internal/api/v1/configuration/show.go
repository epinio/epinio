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

package configuration

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Show handles the API end point /namespaces/:namespace/configurations/:configuration
// It returns the detail information of the named configuration instance
func Show(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	configurationName := c.Param("configuration")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	configuration, err := configurations.Lookup(ctx, cluster, namespace, configurationName)
	if err != nil {
		if err.Error() == "configuration not found" {
			return apierror.ConfigurationIsNotKnown(configurationName)
		}
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	appNames, err := application.BoundAppsNamesFor(ctx, cluster, namespace, configurationName)
	if err != nil {
		return apierror.InternalError(err)
	}

	configurationDetails, err := configuration.Details(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	// For service-based configuration, fetch and record siblings. Itself excluded, of course.
	siblings := []string{}
	if configuration.Origin != "" {
		kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
		if err != nil {
			return apierror.InternalError(err)
		}

		service, err := kubeServiceClient.Get(ctx, namespace, configuration.Origin)
		if err != nil {
			return apierror.InternalError(err)
		}

		serviceConfigurations, err := configurations.ForService(ctx, cluster, service)
		if err != nil {
			return apierror.InternalError(err)
		}

		for _, secret := range serviceConfigurations {
			if secret.Name != configuration.Name {
				siblings = append(siblings, secret.Name)
			}
		}
	}

	response.OKReturn(c, models.ConfigurationResponse{
		Meta: models.ConfigurationRef{
			Meta: models.Meta{
				CreatedAt: configuration.CreatedAt,
				Name:      configuration.Name,
				Namespace: configuration.Namespace(),
			},
		},
		Configuration: models.ConfigurationShowResponse{
			Username:  configuration.User(),
			Details:   configurationDetails,
			BoundApps: appNames,
			Type:      configuration.Type,
			Origin:    configuration.Origin,
			Siblings:  siblings,
		},
	})
	return nil
}
