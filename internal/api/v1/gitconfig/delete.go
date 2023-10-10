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
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	gitbridge "github.com/epinio/epinio/internal/bridge/git"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Delete handles the API endpoint /gitconfigs/:gitconfig (DELETE).
// It destroys the git configuration specified by its name.
func Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	gitconfigName := c.Param("gitconfig")
	logger := requestctx.Logger(ctx)

	var gitconfigNames []string
	gitconfigNames, found := c.GetQueryArray("gitconfigs[]")
	if !found {
		gitconfigNames = append(gitconfigNames, gitconfigName)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	authService := auth.NewAuthService(logger, cluster)

	manager, err := gitbridge.NewManager(logger, cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()))
	if err != nil {
		return apierror.InternalError(err, "creating git configuration manager")
	}

	gitconfigList := manager.Configurations

	// see create.go
	gcNames := map[string]struct{}{}
	for _, gitconfig := range gitconfigList {
		gcNames[gitconfig.ID] = struct{}{}
	}

	for _, gitconfig := range gitconfigNames {
		if _, ok := gcNames[gitconfig]; !ok {
			continue
		}

		// delete the gitconfig from all the Users
		err = authService.RemoveGitconfigFromUsers(ctx, gitconfig)
		if err != nil {
			errDetail := fmt.Sprintf("error removing gitconfig [%s] from users", gitconfig)
			return apierror.InternalError(err, errDetail)
		}

		err := cluster.DeleteSecret(ctx, helmchart.Namespace(), gitconfig)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	response.OK(c)
	return nil
}
