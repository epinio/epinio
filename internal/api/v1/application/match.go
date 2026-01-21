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

package application

import (
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Match handles the API endpoint /namespace/:namespace/appsmatches/:pattern (GET)
// It returns a list of all Epinio-controlled applications matching the prefix pattern.
func Match(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	namespace := c.Param("namespace")

	helpers.Logger.Infow("match applications")
	defer helpers.Logger.Infow("return")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	helpers.Logger.Infow("list applications")

	apps, err := application.ListAppRefs(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	helpers.Logger.Infow("get application prefix")
	prefix := c.Param("pattern")

	helpers.Logger.Infow("match prefix", "pattern", prefix)
	matches := []string{}
	for _, app := range apps {
		if strings.HasPrefix(app.Name, prefix) {
			matches = append(matches, app.Name)
		}
	}

	helpers.Logger.Infow("deliver matches", "found", matches)

	response.OKReturn(c, models.AppMatchResponse{
		Names: matches,
	})
	return nil
}
