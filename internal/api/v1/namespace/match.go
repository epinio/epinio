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

package namespace

import (
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/namespaces"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Match handles the API endpoint /namespacematches/:pattern (GET)
// It returns a list of all Epinio-controlled namespaces matching the prefix pattern.
func Match(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)
	user := requestctx.User(ctx)

	log.Info("match namespaces")
	defer log.Info("return")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Info("list namespaces")
	namespaces, err := namespaces.List(ctx, cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	namespaces = auth.FilterResources(user, namespaces)

	log.Info("get namespace prefix")
	prefix := c.Param("pattern")

	log.Info("match prefix", "pattern", prefix)
	matches := []string{}
	for _, namespace := range namespaces {
		if strings.HasPrefix(namespace.Name, prefix) {
			matches = append(matches, namespace.Name)
		}
	}

	log.Info("deliver matches", "found", matches)

	response.OKReturn(c, models.NamespacesMatchResponse{
		Names: matches,
	})
	return nil
}
