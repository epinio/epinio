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

// Package server provides the Epinio http server
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/middleware"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/domain"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/pkg/errors"

	"github.com/alron/ginlogr"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/mattn/go-colorable"
	"github.com/spf13/viper"
)

// NewHandler creates and setup the gin router
func NewHandler(logger logr.Logger) (*gin.Engine, error) {
	// Support colors on Windows also
	gin.DefaultWriter = colorable.NewColorableStdout()

	gin.SetMode(gin.ReleaseMode)

	// Endpoint structure ...
	// | Path              | Notes      | Logging
	// | ---               | ---        | ----
	// | <Root>/...        | API        | Via "<Root>" Group
	// | /ready            | L/R Probes |
	// | /namespaces/target/:namespace | ditto      | ditto

	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.NoMethod(func(ctx *gin.Context) {
		response.Error(ctx, apierrors.NewAPIError("Method not allowed", http.StatusMethodNotAllowed))
	})
	router.NoRoute(func(ctx *gin.Context) {
		response.Error(ctx, apierrors.NewNotFoundError("route", ctx.Request.URL.Path))
	})

	// Do not set header if nothing is specified.
	accessControlAllowOrigin := strings.TrimSuffix(viper.GetString("access-control-allow-origin"), "/")
	if accessControlAllowOrigin != "" {
		router.Use(func(ctx *gin.Context) {
			ctx.Header("Access-Control-Allow-Origin", accessControlAllowOrigin)
			ctx.Header("Access-Control-Allow-Credentials", "true")
			ctx.Header("Access-Control-Allow-Methods", "POST, PUT, PATCH, GET, OPTIONS, DELETE")          // This cannot be a wildcard when `Access-Control-Allow-Credentials` is true
			ctx.Header("Access-Control-Allow-Headers", "Authorization,x-api-csrf,content-type,file-size") // This cannot be a wildcard when `Access-Control-Allow-Credentials` is true
			ctx.Header("Vary", "Origin")                                                                  // Required when `Access-Control-Allow-Origin` is not a wildcard value

			if ctx.Request.Method == "OPTIONS" {
				// OPTIONS requests don't support `Authorization` headers, so return before we hit any checks
				ctx.AbortWithStatus(http.StatusNoContent)
				return
			}
		})
	}

	ginLogger := ginlogr.Ginlogr(logger, time.RFC3339, true)

	// Register routes - No authentication, no logging, no session.
	// This is the healthcheck.
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})
	// And the API self-description
	router.GET("/api/swagger.json", swaggerHandler)

	// Add common middlewares to all the routes declared after
	router.Use(
		ginLogger,
		middleware.Recovery,
		middleware.InitContext(logger),
	)

	// No authentication, no session. This is epinio's version and auth information.
	router.GET("/api/v1/info",
		middleware.EpinioVersion,
		apiv1.ErrorHandler(apiv1.Info),
	)

	// authenticated /me endpoint returns the current user (no other checks/middlewares needed)
	router.GET("/api/v1/me",
		middleware.Authentication,
		middleware.EpinioVersion,
		apiv1.ErrorHandler(apiv1.Me),
	)

	// Dex or no dex ?
	if _, err := os.Stat(apiv1.DexPEMPath); err == nil {
		// dex secret is present, load contained cert

		err := auth.ExtendLocalTrustFromFile(apiv1.DexPEMPath)
		if err != nil {
			return nil, errors.Wrap(err, "extending local trust with dex")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		// No dex secret/file, do nothing
	} else {
		// Some other Stat error, report
		return nil, errors.Wrap(err, "extending local trust with dex")
	}

	// Register api routes
	{
		apiRoutesGroup := router.Group(apiv1.Root,
			middleware.Authentication,
			middleware.EpinioVersion,
			middleware.NamespaceExists,
			middleware.RoleAuthorization,
			middleware.NamespaceAuthorization,
			middleware.GitconfigAuthorization,
		)
		apiv1.Lemon(apiRoutesGroup)
	}

	// Register web socket routes
	{
		wapiRoutesGroup := router.Group(apiv1.WsRoot,
			middleware.TokenAuth,
			middleware.EpinioVersion,
			middleware.NamespaceExists,
			middleware.RoleAuthorization,
			middleware.NamespaceAuthorization,
			// gitconfig has no websocket routes
		)
		apiv1.Spice(wapiRoutesGroup)
	}

	cluster, err := kubernetes.GetCluster(context.Background())
	if err != nil {
		return nil, err
	}
	authservice := auth.NewAuthService(logger, cluster)

	if err := apiv1.InitAuthAndRoles(authservice); err != nil {
		return nil, errors.Wrap(err, "initializing authentication")
	}

	// print all registered routes
	if logger.V(3).Enabled() {
		for _, h := range router.Routes() {
			logger.V(3).Info(fmt.Sprintf("%-6s %s", h.Method, h.Path))
		}
	}

	return router, nil
}

func swaggerHandler(c *gin.Context) {
	swaggerFile, err := os.Open("swagger.json")
	if err != nil {
		response.Error(c, apierrors.InternalError(err))
		c.Abort()
		return
	}

	var swaggerMap map[string]interface{}
	err = json.NewDecoder(swaggerFile).Decode(&swaggerMap)
	if err != nil {
		response.Error(c, apierrors.InternalError(err))
		c.Abort()
		return
	}

	mainDomain, err := domain.MainDomain(c.Request.Context())
	if err != nil {
		response.Error(c, apierrors.InternalError(err))
		c.Abort()
		return
	}
	swaggerMap["host"] = "epinio." + mainDomain

	c.JSON(http.StatusOK, swaggerMap)
}
