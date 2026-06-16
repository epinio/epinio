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
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	gitbridge "github.com/epinio/epinio/internal/bridge/git"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Delete handles the API endpoint /gitconfigs/:gitconfig (DELETE).
// It destroys the git configuration specified by its name.
func Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	user := requestctx.User(ctx)
	gitconfigName := c.Param("gitconfig")

	var gitconfigNames []string
	gitconfigNames, found := c.GetQueryArray("gitconfigs[]")
	if !found {
		gitconfigNames = append(gitconfigNames, gitconfigName)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	authService := auth.NewAuthService(cluster)

	manager, err := gitbridge.NewManager(cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()))
	if err != nil {
		return apierror.InternalError(err, "creating git configuration manager")
	}

	// Index every known config by ID, for existence and ownership checks.
	configsByID := map[string]gitbridge.Configuration{}
	for _, gitconfig := range manager.Configurations {
		configsByID[gitconfig.ID] = gitconfig
	}

	// Validate the whole batch before deleting anything: the user must be allowed
	// to delete every requested config that exists. Global grants read, not delete.
	for _, gitconfig := range gitconfigNames {
		config, exists := configsByID[gitconfig]
		if !exists {
			continue
		}
		if !auth.CanDeleteGitconfig(user, config) {
			return apierror.NewAPIError(
				fmt.Sprintf("user unauthorized to delete gitconfig [%s]", gitconfig),
				http.StatusForbidden,
			)
		}
	}

	for _, gitconfig := range gitconfigNames {
		if _, exists := configsByID[gitconfig]; !exists {
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
