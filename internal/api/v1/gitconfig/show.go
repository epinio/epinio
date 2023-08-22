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

package gitconfig

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	gitbridge "github.com/epinio/epinio/internal/bridge/git"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Show handles the API endpoint GET /gitconfigs/:gitconfig
// It returns the details of the specified git configuration
func Show(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	gitconfigID := c.Param("gitconfig")
	logger := requestctx.Logger(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	manager, err := gitbridge.NewManager(logger, cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()))
	if err != nil {
		return apierror.InternalError(err, "creating git configuration manager")
	}

	gitconfigList := manager.Configurations

	for _, gitconfig := range gitconfigList {
		if gitconfig.ID != gitconfigID {
			continue
		}

		response.OKReturn(c, models.Gitconfig{
			Meta: models.MetaLite{
				Name: gitconfig.ID,
				// CreatedAt: -- Not tracked by gitconfig
			},
			URL:        gitconfig.URL,
			Provider:   gitconfig.Provider,
			Username:   gitconfig.Username,
			UserOrg:    gitconfig.UserOrg,
			Repository: gitconfig.Repository,
			SkipSSL:    gitconfig.SkipSSL,
			// Password    string - Private, excluded
			// Certificate []byte - Private, excluded
		})
		return nil
	}

	return apierror.NewNotFoundError("gitconfig", gitconfigID)
}
