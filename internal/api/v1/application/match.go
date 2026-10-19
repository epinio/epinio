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

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Match handles the API endpoint /namespace/:namespace/appsmatches/:pattern (GET)
// It returns a list of all Epinio-controlled applications matching the prefix pattern.
// Never-deployed (undeployed) apps are omitted so matches align with the Apps list.
func Match(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")

	log.Infow("match applications")
	defer log.Infow("return")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Infow("list applications")

	appClient, err := cluster.ClientApp()
	if err != nil {
		return apierror.InternalError(err)
	}

	list, err := appClient.Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Infow("get application prefix")
	prefix := c.Param("pattern")

	log.Infow("match prefix", "pattern", prefix)
	matches := []string{}
	for i := range list.Items {
		cr := &list.Items[i]
		if !application.MatchesListFilter(cr, application.ListFilterDeployed) {
			continue
		}
		if strings.HasPrefix(cr.GetName(), prefix) {
			matches = append(matches, cr.GetName())
		}
	}

	log.Infow("deliver matches", "found", matches)

	response.OKReturn(c, models.AppMatchResponse{
		Names: matches,
	})
	return nil
}
