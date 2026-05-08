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

// Index handles GET /builderimages — lists all known builderimages.
func Index(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	client, clientError := cluster.ClientBuilderImage()
	if clientError != nil {
		return apierror.InternalError(clientError)
	}

	all, listError := builderimage.List(ctx, client)
	if listError != nil {
		return apierror.InternalError(listError)
	}

	page, pageSize, ok := response.GetPaginationParams(c, 1, 25)
	if ok {
		paged := response.PaginateSlice(all, page, pageSize)
		response.OKReturn(c, paged)
		return nil
	}

	response.OKReturn(c, all)
	return nil
}
