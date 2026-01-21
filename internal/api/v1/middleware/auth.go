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

	"github.com/epinio/epinio/helpers"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/gin-gonic/gin"

	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
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
	logger := helpers.Logger.With("component", "AuthorizationMiddleware")
	user := requestctx.User(c.Request.Context())

	method := c.Request.Method
	path := c.Request.URL.Path

	roleIDs := strings.Join(user.Roles.IDs(), ",")
	logger.Infow("authorization request",
		"user", user.Username,
		"roles", roleIDs,
		"method", method,
		"path", path,
	)

	if user.IsAdmin() {
		logger.Debugw("user [admin] is authorized", "component", "authorizeAdmin")
		return
	}

	// not an admin, check if path is restricted
	if restrictedPath(path) {
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
		authorized := authorizeUser(label, rsrc, allowed)

		roleIDs := strings.Join(user.Roles.IDs(), ",")
		logger.Infow("authorization result",
			"user", user.Username,
			"roles", roleIDs,
			"authorized", authorized,
			"resource_type", label,
			"resource", rsrc,
		)

		if !authorized {
			err := apierrors.NewAPIError(fmt.Sprintf("user unauthorized for %s %s", label, rsrc), http.StatusForbidden)
			response.Error(c, err)
			c.Abort()
			return
		}
	}
}

func authorizeUser(label, resource string, allowed []string) bool {
	logger := helpers.Logger.With("component", "authorizeUser")

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

	logger.Infow("resource not in allowed list",
		"resource_type", label,
		"resource", resource,
		"allowed", strings.Join(allowed, ", "))
	return false
}

func restrictedPath(path string) bool {
	logger := helpers.Logger.With("component", "unrestrictedPath")

	// check if the requested path is restricted
	if _, found := v1.AdminRoutes[path]; found {
		logger.Infow("path is an admin route, user unauthorized", "path", path)
		return true
	}

	// path is free to use
	return false
}
