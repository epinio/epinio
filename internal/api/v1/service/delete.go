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
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// Delete handles the API end point /namespaces/:namespace/services/:service (DELETE)
// It deletes the named service
func Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx).WithName("ServiceDelete")
	username := requestctx.User(ctx).Username

	namespace := c.Param("namespace")

	// TODO: We keep this parameter for now, to not break the API.  We can remove it as soon as
	// all front ends are adapted to the array parameter below.
	serviceName := c.Param("service")

	var serviceNames []string
	serviceNames, found := c.GetQueryArray("services[]")
	if !found {
		serviceNames = append(serviceNames, serviceName)
	}

	var deleteRequest models.ServiceDeleteRequest
	err := c.BindJSON(&deleteRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Collect and validate the referenced services

	theServices := []*models.Service{}
	for _, serviceName := range serviceNames {
		// Note: Validation of the service, i.e. checking for their helm release is
		// (unfortunately) a step to far in checking. Doing so prevents us from deleting
		// partially created services, i.e. those whose deployment was interupted after
		// creation of the main structure, before the creation of the helm release.

		service, apiErr := GetService(ctx, cluster, logger, namespace, serviceName)
		if apiErr != nil {
			return apiErr
		}

		theServices = append(theServices, service)
	}

	logger.Info("services to delete", "services", serviceNames)

	// Collect the configurations per service, and record per bound app the service/config
	// information
	//
	// A service has one or more associated secrets containing its attributes.  Binding turned
	// these secrets into configurations and bound them to the application.  Unbinding simply
	// unbound them.  We may think that this means that we only have to look for the first
	// configuration to determine what apps the service is bound to. Not so. With the secrets
	// visible as configurations an adventurous user may have unbound them in part, and left in
	// part. So, check everything, and then de-duplicate.

	type appInfo struct {
		service string
		config  v1.Secret
	}

	appConfigurationsMap := make(map[string][]appInfo)
	boundAppNames := []string{}

	for _, service := range theServices {
		serviceConfigurations, err := configurations.ForService(ctx, cluster, service)
		if err != nil {
			return apierror.InternalError(err)
		}

		logger.Info("configurations", "service", service.Meta.Name, "count", len(serviceConfigurations))

		for _, secret := range serviceConfigurations {
			logger.Info("configuration secret", "service", service.Meta.Name, "secret", secret.Name)

			bound, err := application.BoundAppsNamesFor(ctx, cluster, namespace, secret.Name)
			if err != nil {
				return apierror.InternalError(err)
			}

			// inverted lookup map:{ appName: [info1, info2,...] }
			//
			// Note that the configs for a service are spread over the collected appInfos.
			// They are properly merged later, when it comes to actual automatic unbinding.
			for _, appName := range bound {
				appConfigurationsMap[appName] = append(appConfigurationsMap[appName], appInfo{
					service: service.Meta.Name,
					config:  secret,
				})
			}
			boundAppNames = append(boundAppNames, bound...)
		}
	}

	boundAppNames = helpers.UniqueStrings(boundAppNames)

	logger.Info("bound to", "apps", boundAppNames, "count", len(boundAppNames), "unbind", deleteRequest.Unbind)

	// Verify that the services are unbound. IOW not bound to any application.  If they are, and
	// automatic unbind was requested, do that.  Without automatic unbind such applications are
	// reported as error.

	if len(boundAppNames) > 0 {
		if !deleteRequest.Unbind {
			return apierror.NewBadRequestError("bound applications exist").
				WithDetails(strings.Join(boundAppNames, ","))
		}

		logger.Info("app/configuration linkage", "map", appConfigurationsMap)

		// Unbind all the services' configurations from the found applications.  Using the
		// inverted map holding the service/config information per app
		for appName, infos := range appConfigurationsMap {
			logger.Info("unbind from", "app name", appName)

			// Note: Merge the configs per service
			infoMap := make(map[string][]v1.Secret)
			for _, info := range infos {
				infoMap[info.service] = append(infoMap[info.service], info.config)
			}

			// ... And run the unbind per app and service.
			for serviceName, serviceConfigurations := range infoMap {
				logger.Info("unbinding of", "app", appName, "service", serviceName)

				apiErr := UnbindService(ctx, cluster, logger, namespace, serviceName,
					appName, username, serviceConfigurations)
				if apiErr != nil {
					return apiErr
				}
			}
		}
	}

	// Finally, delete the services

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	for _, serviceName := range serviceNames {
		err = kubeServiceClient.Delete(ctx, namespace, serviceName)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return apierror.NewNotFoundError("service", serviceName).WithDetailsf(err.Error())
			}

			return apierror.InternalError(err)
		}
	}

	response.OKReturn(c, models.ServiceDeleteResponse{
		BoundApps: boundAppNames,
	})
	return nil
}
