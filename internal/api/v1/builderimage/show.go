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

package builderimage

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/builderimage"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Show handles GET /builderimages/:name — returns details of the named builderimage.
func Show(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	name := c.Param("name")

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	builderimage, builderimageError := builderimage.Lookup(ctx, cluster, name)
	if builderimageError != nil {
		return apierror.InternalError(builderimageError)
	}

	if builderimage == nil {
		return apierror.BuilderImageIsNotKnown(name)
	}

	response.OKReturn(c, builderimage)
	return nil
}
