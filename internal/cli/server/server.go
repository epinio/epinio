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

	"github.com/epinio/epinio/helpers/authtoken"
	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/domain"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"

	"github.com/alron/ginlogr"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
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
	router.Use(gin.Recovery())

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
	ginRecoveryLogger := ginlogr.RecoveryWithLogr(logger, time.RFC3339, true, true)

	// Register routes
	// No authentication, no logging, no session. This is the healthcheck.
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	router.GET("/api/swagger.json", swaggerHandler)

	// add common middlewares to all the routes
	router.Use(
		ginLogger,
		ginRecoveryLogger,
		initContextMiddleware(logger),
	)

	// Register api routes
	{
		apiRoutesGroup := router.Group(apiv1.Root,
			authMiddleware,
			apiv1.NamespaceMiddleware,
			apiv1.AuthorizationMiddleware,
		)
		apiv1.Lemon(apiRoutesGroup)
	}

	// Register web socket routes
	{
		wapiRoutesGroup := router.Group(apiv1.WsRoot,
			tokenAuthMiddleware,
			apiv1.NamespaceMiddleware,
			apiv1.AuthorizationMiddleware,
		)
		apiv1.Spice(wapiRoutesGroup)
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

// initContextMiddleware initialize the Request Context injecting the logger and the requestID
func initContextMiddleware(logger logr.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reqCtx := ctx.Request.Context()

		requestID := uuid.NewString()
		baseLogger := logger.WithValues("requestId", requestID)

		reqCtx = requestctx.WithID(reqCtx, requestID)
		reqCtx = requestctx.WithLogger(reqCtx, baseLogger)
		ctx.Request = ctx.Request.WithContext(reqCtx)
	}
}

// authMiddleware authenticates the user either using the session or if one
// doesn't exist, it authenticates with basic auth.
func authMiddleware(ctx *gin.Context) {
	reqCtx := ctx.Request.Context()
	logger := requestctx.Logger(reqCtx).WithName("AuthMiddleware")

	userMap, err := loadUsersMap(ctx)
	if err != nil {
		response.Error(ctx, apierrors.InternalError(err))
		ctx.Abort()
		return
	}

	if len(userMap) == 0 {
		response.Error(ctx, apierrors.NewAPIError("no user found", http.StatusUnauthorized))
		ctx.Abort()
		return
	}

	logger.V(1).Info("Basic auth authentication")

	// we need this check to return a 401 instead of an error
	auth := ctx.Request.Header.Get("Authorization")
	if auth == "" {
		response.Error(ctx, apierrors.NewAPIError("missing credentials", http.StatusUnauthorized))
		ctx.Abort()
		return
	}

	username, password, ok := ctx.Request.BasicAuth()
	if !ok {
		response.Error(ctx, apierrors.NewInternalError("Couldn't extract user from the auth header"))
		ctx.Abort()
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(userMap[username].Password), []byte(password))
	if err != nil {
		response.Error(ctx, apierrors.NewAPIError("wrong password", http.StatusUnauthorized))
		ctx.Abort()
		return
	}

	// We set this to the current user after successful authentication.
	// This is also added to the context for controllers to use.
	user := userMap[username]

	// Write the user info in the context. It's needed by the next middleware
	// to write it into the session.
	newCtx := ctx.Request.Context()
	newCtx = requestctx.WithUser(newCtx, user)
	ctx.Request = ctx.Request.Clone(newCtx)
}

func loadUsersMap(ctx context.Context) (map[string]auth.User, error) {
	authService, err := auth.NewAuthServiceFromContext(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create auth service from context")
	}

	users, err := authService.GetUsers(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get users")
	}

	userMap := make(map[string]auth.User)
	for _, user := range users {
		userMap[user.Username] = user
	}

	return userMap, nil
}

// tokenAuthMiddleware is only used to establish websocket connections for authenticated users
func tokenAuthMiddleware(ctx *gin.Context) {
	logger := requestctx.Logger(ctx.Request.Context()).WithName("TokenAuthMiddleware")
	logger.V(1).Info("Authtoken authentication")

	token := ctx.Query("authtoken")
	claims, err := authtoken.Validate(token)
	if err != nil {
		apiErr := apierrors.NewAPIError("unknown token validation error", http.StatusUnauthorized)
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				apiErr.Title = "malformed token format"

			} else if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
				apiErr.Title = "token expired"

			} else {
				apiErr.Title = "cannot handle token"
			}
		}

		// detailed log message
		logger.V(2).Info(apiErr.Title, "error", err.Error())
		// not too specific log message for unauthorized client
		response.Error(ctx, apiErr)
		ctx.Abort()
		return
	}

	authService, err := auth.NewAuthServiceFromContext(ctx)
	if err != nil {
		response.Error(ctx, apierrors.InternalError(err))
		ctx.Abort()
		return
	}

	// find the user and add it in the context
	users, err := authService.GetUsers(ctx)
	if err != nil {
		response.Error(ctx, apierrors.InternalError(err))
		ctx.Abort()
		return
	}

	for _, user := range users {
		if user.Username == claims.Username {
			newCtx := ctx.Request.Context()
			newCtx = requestctx.WithUser(newCtx, user)
			ctx.Request = ctx.Request.Clone(newCtx)

			break
		}
	}
}
