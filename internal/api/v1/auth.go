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

package v1

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
)

func AuthorizationMiddleware(c *gin.Context) {
	logger := requestctx.Logger(c.Request.Context()).WithName("AuthorizationMiddleware")
	user := requestctx.User(c.Request.Context())

	method := c.Request.Method
	path := c.Request.URL.Path
	namespace := c.Param("namespace")

	logger.Info(fmt.Sprintf("authorization request from user [%s] with role [%s] for [%s - %s]", user.Username, user.Role, method, path))

	var authorized bool
	switch user.Role {
	case "admin":
		authorized = authorizeAdmin(logger)
	case "user":
		authorized = authorizeUser(logger, user, path, namespace)
	}

	logger.Info(fmt.Sprintf("user [%s] with role [%s] authorized [%t] for namespace [%s]", user.Username, user.Role, authorized, namespace))

	if !authorized {
		response.Error(c, apierrors.NewAPIError("user unauthorized", http.StatusForbidden))
		c.Abort()
	}

}

func authorizeAdmin(logger logr.Logger) bool {
	logger.V(1).WithName("authorizeAdmin").Info("user admin is authorized")
	return true
}

func authorizeUser(logger logr.Logger, user auth.User, path, namespace string) bool {
	logger = logger.V(1).WithName("authorizeUser")

	// check if the requested path is restricted
	if _, found := AdminRoutes[path]; found {
		logger.Info(fmt.Sprintf("path [%s] is an admin route, user unauthorized", path))
		return false
	}

	// check if the user has permission on the requested namespace
	if namespace != "" {
		for _, ns := range user.Namespaces {
			if namespace == ns {
				return true
			}
		}

		logger.Info(fmt.Sprintf("namespace [%s] is not in user namespaces [%s]", namespace, strings.Join(user.Namespaces, ", ")))
		return false
	}

	// all non-admin routes are public
	return true
}
