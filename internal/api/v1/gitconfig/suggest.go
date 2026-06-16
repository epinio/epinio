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
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Suggest handles the API endpoint GET /gitconfigssuggest?url=<gitURL>
// It returns the name of the most specific git configuration the user is allowed
// to use for the given repository URL, or an empty name when none matches. This is
// only a suggestion for the UI to pre-select; the import endpoint still requires an
// explicit choice.
func Suggest(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	user := requestctx.User(ctx)

	gitURL := c.Query("url")
	if gitURL == "" {
		return apierror.NewBadRequestError("missing url query parameter")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	manager, managerError := gitbridge.
		NewManager(cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()))
	if managerError != nil {
		return apierror.InternalError(
			managerError,
			"creating git configuration manager",
		)
	}

	// Only ever suggest configurations the user is permitted to use.
	allowed := auth.FilterGitconfigResources(user, manager.Configurations)
	scoped := &gitbridge.Manager{Configurations: allowed}

	gitConfig, findError := scoped.FindConfiguration(gitURL)
	if findError != nil {
		return apierror.InternalError(
			findError,
			fmt.Sprintf("finding git configuration for gitURL [%s]", gitURL),
		)
	}

	name := ""
	if gitConfig != nil {
		name = gitConfig.ID
	}

	response.OKReturn(c, models.GitconfigSuggestResponse{Name: name})
	return nil
}
