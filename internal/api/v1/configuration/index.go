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
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Index handles the API end point /namespaces/:namespace/configurations
// It returns a list of all known configuration instances
func Index(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	namespaceConfigurations, err := configurations.List(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	appsOf, err := application.BoundAppsNames(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	responseData, err := makeResponse(ctx, appsOf, namespaceConfigurations)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, responseData)
	return nil
}

func makeResponse(ctx context.Context, appsOf map[application.ConfigurationKey][]string, configurations configurations.ConfigurationList) (models.ConfigurationResponseList, error) {

	response := models.ConfigurationResponseList{}

	// All the possible siblings of a configuration are present in the configuration list.
	//
	// Simply iterate and sort them into buckets by service origin. Note that the namespace has
	// to be part of the key. Non-service configurations are ignored.

	siblingMap := map[string][]string{}
	for _, configuration := range configurations {
		if configuration.Origin != "" {
			key := fmt.Sprintf("n%s/o%s", configuration.Namespace(), configuration.Origin)
			siblingMap[key] = append(siblingMap[key], configuration.Name)
		}
	}

	for _, configuration := range configurations {
		configurationDetails, err := configuration.Details(ctx)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue // Configuration was deleted, ignore it
			} else {
				return models.ConfigurationResponseList{}, err
			}
		}

		key := application.EncodeConfigurationKey(configuration.Name, configuration.Namespace())
		appNames := appsOf[key]

		// For service-based configuration, pull siblings out of the map. Itself excluded, of course.

		siblings := []string{}
		if configuration.Origin != "" {
			key := fmt.Sprintf("n%s/o%s", configuration.Namespace(), configuration.Origin)
			for _, maybeSibling := range siblingMap[key] {
				if maybeSibling != configuration.Name {
					siblings = append(siblings, maybeSibling)
				}
			}
		}

		response = append(response, models.ConfigurationResponse{
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
	}

	return response, nil
}
