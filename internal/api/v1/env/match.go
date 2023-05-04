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

package env

import (
	"sort"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Match handles the API endpoint /namespaces/:namespace/applications/:app/environment/:env/match/:pattern
// It receives the namespace, application name, plus a prefix and returns
// the names of all the environment associated with that application
// with prefix
func Match(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespaceName := c.Param("namespace")
	appName := c.Param("app")
	prefix := c.Param("pattern")

	log.Info("returning matching environment variable names",
		"namespace", namespaceName, "app", appName, "prefix", prefix)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	app := models.NewAppRef(appName, namespaceName)

	exists, err := application.Exists(ctx, cluster, app)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.AppIsNotKnown(appName)
	}

	// EnvList, with post-processing - selection of matches, and
	// projection to deliver only names

	environment, err := application.Environment(ctx, cluster, app)
	if err != nil {
		return apierror.InternalError(err)
	}

	matches := []string{}
	for evName := range environment {
		if strings.HasPrefix(evName, prefix) {
			matches = append(matches, evName)
		}
	}
	sort.Strings(matches)

	response.OKReturn(c, models.EnvMatchResponse{
		Names: matches,
	})
	return nil
}
