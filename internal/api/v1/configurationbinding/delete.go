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

package configurationbinding

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Delete handles the API endpoint /namespaces/:namespace/applications/:app/configurationbindings/:configuration
// It removes the binding between the specified configuration and application
func Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	configurationName := c.Param("configuration")
	username := requestctx.User(ctx).Username

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	config, err := configurations.Lookup(ctx, cluster, namespace, configurationName)
	if err != nil && err.Error() == "configuration not found" {
		return apierror.ConfigurationIsNotKnown(configurationName)
	}
	if err != nil {
		return apierror.InternalError(err)
	}

	if config.Origin != "" {
		// [BELONG] keep in sync with same marker in the client
		return apierror.NewBadRequestErrorf("Configuration '%s' belongs to service '%s', use service requests",
			config.Name,
			config.Origin)
	}

	apiErr := DeleteBinding(ctx, cluster, namespace, appName, username, []string{configurationName})
	if apiErr != nil {
		return apiErr
	}

	response.OK(c)
	return nil
}
