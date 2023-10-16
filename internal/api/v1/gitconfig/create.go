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
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/gin-gonic/gin"
)

// Create handles the API endpoint /gitconfigs (POST).
// It creates a gitconfig with the specified name.
func Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx)
	user := requestctx.User(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	authService := auth.NewAuthService(logger, cluster)

	var request models.GitconfigCreateRequest
	err = c.BindJSON(&request)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	gitconfigName := request.ID
	if gitconfigName == "" {
		return apierror.NewBadRequestError("name of gitconfig to create not found")
	}
	errorMsgs := validation.IsDNS1123Subdomain(gitconfigName)
	if len(errorMsgs) > 0 {
		return apierror.NewBadRequestErrorf("Git configurations' name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name', or '123-abc').")
	}

	manager, err := gitbridge.NewManager(logger, cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()))
	if err != nil {
		return apierror.InternalError(err, "creating git configuration manager")
	}

	gitconfigList := manager.Configurations

	// see delete.go
	gcNames := map[string]struct{}{}
	for _, gitconfig := range gitconfigList {
		gcNames[gitconfig.ID] = struct{}{}
	}

	// already known ?
	if _, ok := gcNames[gitconfigName]; ok {
		return apierror.NewConflictError("gitconfig", gitconfigName)
	}

	secret := gitbridge.NewSecretFromConfiguration(gitbridge.Configuration{
		ID:          request.ID,
		URL:         request.URL,
		Provider:    request.Provider,
		Username:    request.Username,
		Password:    request.Password,
		UserOrg:     request.UserOrg,
		Repository:  request.Repository,
		SkipSSL:     request.SkipSSL,
		Certificate: request.Certificates,
	})

	err = cluster.CreateSecret(ctx, helmchart.Namespace(), secret)
	if err != nil {
		return apierror.InternalError(err)
	}

	// add the gitconfig to the User's gitconfigs
	user.AddGitconfig(request.ID)
	user, err = authService.UpdateUser(ctx, user)
	if err != nil {
		errDetail := fmt.Sprintf("error adding gitconfig [%s] to user [%s]", request.ID, user.Username)
		return apierror.InternalError(err, errDetail)
	}

	response.Created(c)
	return nil
}
