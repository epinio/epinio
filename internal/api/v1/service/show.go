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

	// Retrieve the names of the applications the service is bound to.

	appNames, err := application.ServicesBoundAppsNamesFor(ctx, cluster, namespace, serviceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Retrieve the configurations (Kube secrets) the service chart created to contain
	// credentials and other details. Note that a service can create more than one
	// configuration/secret. An example for this is RabbitMQ.

	serviceConfigurations, err := configurations.ForService(ctx, cluster, service)
	if err != nil {
		return apierror.InternalError(err)
	}

	//

	if len(serviceConfigurations) > 0 {
		// We have associated secrets/configurations. We put the data from all the
		// configurations into a __single__ map to be returned to the user. As we use the
		// data keys from the secrets as they keys of the returned map it is possible that
		// we run into a conflict, i.e. two secrets using the same key for their data.
		// While this could be avoided by adding the name of the secret to the result key
		// this would result into very long, and pretty unreadable keys.
		// The conflicts are resolved by adding an integer sequence number to the key.
		//
		// Examples:
		//   - `foo` and `foo.1` for a single conflict between two keys.
		//   - `bar`, `bar.1`, `bar.2`, etc. for a conflict with more than 2 keys.

		service.Details = map[string]string{}
		for _, serviceConfig := range serviceConfigurations {
			for key, value := range serviceConfig.Data {
				if _, ok := service.Details[key]; ok {
					// Key conflict. The same key was set by a previous secret.
					// Find an integer to append to the base key creating a key
					// which is not yet used. This is a loop because additional
					// preceding conflicts may have already used a key with integer.
					for j := 0; true; j++ {
						xkey := fmt.Sprintf("%s.%d", key, j)
						if _, ok := service.Details[xkey]; !ok {
							// Conflict resolved, key + this serial not yet used.
							service.Details[key] = string(value)
							break
						}
						// Still in conflict, continue to next serial
					}
					// Break above comes to here, conflict resolved, continue with next key
					continue
				}
				// No conflict
				service.Details[key] = string(value)
			}
		}
	}

	service.BoundApps = appNames

	response.OKReturn(c, service)
	return nil
}
