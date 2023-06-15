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
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Create handles the API end point /namespaces/:namespace/configurations
// It creates the named configuration from its parameters
func Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	username := requestctx.User(ctx).Username

	var createRequest models.ConfigurationCreateRequest
	err := c.BindJSON(&createRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	if createRequest.Name == "" {
		return apierror.NewBadRequestError("cannot create configuration without a name")
	}

	errorMsgs := validation.IsDNS1123Subdomain(createRequest.Name)
	if len(errorMsgs) > 0 {
		return apierror.NewBadRequestErrorf("Configuration's name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name', or '123-abc').")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Verify that the requested name is not yet used by a different configuration.
	_, err = configurations.Lookup(ctx, cluster, namespace, createRequest.Name)
	if err == nil {
		// no error, configuration is found, conflict
		return apierror.ConfigurationAlreadyKnown(createRequest.Name)
	}
	if err != nil && err.Error() != "configuration not found" {
		// some internal error
		return apierror.InternalError(err)
	}
	// any error here is `configuration not found`, and we can continue

	// Create the new configuration. At last.
	_, err = configurations.CreateConfiguration(ctx, cluster, createRequest.Name, namespace, username, createRequest.Data)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.Created(c)
	return nil
}
