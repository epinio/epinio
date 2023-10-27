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

package middleware

import (
	"fmt"
	"net/http"
	"strings"

	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
)

// RoleAuthorization middleware is used to check if the user is allowed for the incoming request
// checking the verb, path and eventually the params of it.
func RoleAuthorization(c *gin.Context) {
	user := requestctx.User(c.Request.Context())

	params := map[string]string{}
	for _, p := range c.Params {
		params[p.Key] = p.Value
	}

	allowed := user.IsAllowed(c.Request.Method, c.FullPath(), params)

	if !allowed {
		err := apierrors.NewAPIError("user unauthorized", http.StatusForbidden)
		response.Error(c, err)
		c.Abort()
		return
	}
}

func NamespaceAuthorization(c *gin.Context) {
	user := requestctx.User(c.Request.Context())
	authorization(c, "namespace", user.Namespaces)
}

func GitconfigAuthorization(c *gin.Context) {
	user := requestctx.User(c.Request.Context())
	authorization(c, "gitconfig", user.Gitconfigs)
}

func authorization(c *gin.Context, label string, allowed []string) {
	logger := requestctx.Logger(c.Request.Context()).WithName("AuthorizationMiddleware")
	user := requestctx.User(c.Request.Context())

	method := c.Request.Method
	path := c.Request.URL.Path

	roleIDs := strings.Join(user.Roles.IDs(), ",")
	logger.Info(fmt.Sprintf(
		"authorization request from user [%s] with roles [%s] for [%s - %s]",
		user.Username, roleIDs, method, path,
	))

	if user.IsAdmin() {
		logger.V(1).WithName("authorizeAdmin").Info("user [admin] is authorized")
		return
	}

	// not an admin, check if path is restricted
	if restrictedPath(logger, path) {
		err := apierrors.NewAPIError("user unauthorized, path restricted", http.StatusForbidden)
		response.Error(c, err)
		c.Abort()
		return
	}

	// extract the resources
	resourceName := c.Param(label)
	var resourceNames []string
	resourceNames, found := c.GetQueryArray(label + "s[]")
	if !found {
		resourceNames = append(resourceNames, resourceName)
	}

	for _, rsrc := range resourceNames {
		authorized := authorizeUser(logger, label, rsrc, allowed)

		roleIDs := strings.Join(user.Roles.IDs(), ",")
		logger.Info(fmt.Sprintf(
			"user [%s] with roles [%s] authorized [%t] for %s [%s]",
			user.Username, roleIDs, authorized, label, rsrc,
		))

		if !authorized {
			err := apierrors.NewAPIError(fmt.Sprintf("user unauthorized for %s %s", label, rsrc), http.StatusForbidden)
			response.Error(c, err)
			c.Abort()
			return
		}
	}
}

func authorizeUser(logger logr.Logger, label, resource string, allowed []string) bool {
	logger = logger.V(1).WithName("authorizeUser")

	// check if the user has permission on the requested resource
	if resource == "" {
		// empty resource always permitted
		return true
	}

	for _, ns := range allowed {
		if resource == ns {
			return true
		}
	}

	logger.Info(fmt.Sprintf("%s [%s] is not in user %ss [%s]",
		label, resource, label, strings.Join(allowed, ", ")))
	return false
}

func restrictedPath(logger logr.Logger, path string) bool {
	logger = logger.V(1).WithName("unrestrictedPath")

	// check if the requested path is restricted
	if _, found := v1.AdminRoutes[path]; found {
		logger.Info(fmt.Sprintf("path [%s] is an admin route, user unauthorized", path))
		return true
	}

	// path is free to use
	return false
}
