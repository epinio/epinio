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
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/namespaces"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/gin-gonic/gin"
)

// Create handles the API endpoint /namespaces (POST).
// It creates a namespace with the specified name.
func Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	var request models.NamespaceCreateRequest
	err = c.BindJSON(&request)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}
	namespaceName := request.Name

	if namespaceName == "" {
		return apierror.NewBadRequestError("name of namespace to create not found")
	}

	errorMsgs := validation.IsDNS1123Subdomain(namespaceName)
	if len(errorMsgs) > 0 {
		return apierror.NewBadRequestErrorf("Namespace's name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name', or '123-abc').")
	}

	exists, err := namespaces.Exists(ctx, cluster, namespaceName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if exists {
		return apierror.NamespaceAlreadyKnown(namespaceName)
	}

	err = namespaces.Create(ctx, cluster, namespaceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	err = addNamespaceToUser(ctx, namespaceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.Created(c)
	return nil
}

// addNamespaceToUser will add the namespace to the User namespaces
func addNamespaceToUser(ctx context.Context, namespace string) error {
	user := requestctx.User(ctx)

	authService, err := auth.NewAuthServiceFromContext(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating auth service")
	}

	err = authService.AddNamespaceToUser(ctx, user.Username, namespace)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error adding namespace [%s] to user [%s]", namespace, user.Username))
	}
	return nil
}
