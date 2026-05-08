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
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/builderimage"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Match handles GET /builderimagesmatch/:pattern
func Match(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	log.Infow("match builderimages")
	defer log.Infow("return")

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	log.Infow("list builderimages")
	builderimageList, listError := builderimage.List(ctx, cluster)
	if listError != nil {
		return apierror.InternalError(listError)
	}

	prefix := c.Param("pattern")

	log.Infow("match prefix", "pattern", prefix)
	matches := []string{}
	for _, builderimage := range builderimageList {
		if strings.HasPrefix(builderimage.Meta.Name, prefix) {
			matches = append(matches, builderimage.Meta.Name)
		}
	}

	log.Infow("deliver matches", "found", matches)

	response.OKReturn(c, models.BuilderImageMatchResponse{
		Names: matches,
	})
	return nil
}
