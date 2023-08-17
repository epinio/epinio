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
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

func Show(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	serviceName := c.Param("service")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	service, err := kubeServiceClient.Get(ctx, namespace, serviceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if service == nil {
		return apierror.ServiceIsNotKnown(serviceName)
	}

	appNames, err := application.ServicesBoundAppsNamesFor(ctx, cluster, namespace, serviceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	serviceConfigurations, err := configurations.ForService(ctx, cluster, service)
	if err != nil {
		return apierror.InternalError(err)
	}

	if len(serviceConfigurations) > 0 {
		service.Details = map[string]string{}
		for _, serviceConfig := range serviceConfigurations {
			for key, value := range serviceConfig.Data {
				if _, ok := service.Details[key]; ok {
					for j := 0; true; j++ {
						xkey := fmt.Sprintf("%s.%d", key, j)
						if _, ok := service.Details[xkey]; !ok {
							service.Details[key] = string(value)
							break
						}
					}
					continue
				}
				service.Details[key] = string(value)
			}
		}
	}

	service.BoundApps = appNames

	response.OKReturn(c, service)
	return nil
}
