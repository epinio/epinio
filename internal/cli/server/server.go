// Package server provides the Epinio http server
package server

import (
	"context"
	"encoding/gob"
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
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
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
		response.Error(ctx, apierrors.NewAPIError("Method not allowed", "", http.StatusMethodNotAllowed))
	})
	router.NoRoute(func(ctx *gin.Context) {
		response.Error(ctx, apierrors.NewNotFoundError(errors.New("Route not found")))
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

	if os.Getenv("SESSION_KEY") == "" {
		return nil, errors.New("SESSION_KEY environment variable not defined")
	}

	store := cookie.NewStore([]byte(os.Getenv("SESSION_KEY")))
	store.Options(sessions.Options{MaxAge: 60 * 60 * 24}) // expire in a day
	gob.Register(auth.User{})

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
		sessions.Sessions("epinio-session", store),
		ginLogger,
		ginRecoveryLogger,
		initContextMiddleware(logger),
	)

	// Register api routes
	{
		apiRoutesGroup := router.Group(apiv1.Root,
			authMiddleware,
			sessionMiddleware,
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
		response.Error(ctx, apierrors.NewAPIError("no user found", "", http.StatusUnauthorized))
		ctx.Abort()
		return
	}

	// We set this to the current user after successful authentication.
	// This is also added to the context for controllers to use.
	var user auth.User

	session := sessions.Default(ctx)
	sessionUser := session.Get("user")
	if sessionUser != nil {
		logger.V(1).Info("Session authentication")

		userInSession, ok := sessionUser.(auth.User)
		if !ok {
			response.Error(ctx, apierrors.NewInternalError("Couldn't parse user from session"))
			ctx.Abort()
			return
		}

		// Check if that user still exists. If not delete the session and block the request!
		// This allows us to kick out users even if they keep their browser open.
		_, found := userMap[userInSession.Username]
		if !found {
			session.Clear()
			session.Options(sessions.Options{MaxAge: -1})

			if err := session.Save(); err != nil {
				response.Error(ctx, apierrors.NewInternalError("Couldn't save the session"))
				ctx.Abort()
				return
			}

			response.Error(ctx, apierrors.NewAPIError("User no longer exists. Session expired.", "", http.StatusUnauthorized))
			ctx.Abort()
			return
		}

		user = userInSession

	} else {
		// no session exists, try basic auth
		logger.V(1).Info("Basic auth authentication")

		// we need this check to return a 401 instead of an error
		auth := ctx.Request.Header.Get("Authorization")
		if auth == "" {
			response.Error(ctx, apierrors.NewAPIError("missing credentials", "", http.StatusUnauthorized))
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
			response.Error(ctx, apierrors.NewAPIError("wrong password", "", http.StatusUnauthorized))
			ctx.Abort()
			return
		}

		user = userMap[username]
	}

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

// sessionMiddleware creates a new session for a logged in user.
// This middleware is not called when authentication fails. That's because
// the authMiddleware calls "ctx.Abort()" in that case.
// We only set the user in session upon successful authentication
// (either basic auth or cookie based).
func sessionMiddleware(ctx *gin.Context) {
	session := sessions.Default(ctx)
	requestContext := ctx.Request.Context()

	user := requestctx.User(requestContext)
	if user.Username == "" { // This can't be, authentication has succeeded.
		response.Error(ctx, apierrors.NewInternalError("Couldn't set user in session after successful authentication. This can't happen."))
		ctx.Abort()
		return
	}

	if session.Get("user") == nil { // Only the first time after authentication success

		// remove the Password from the user saved in session (just in case)
		user.Password = ""

		session.Set("user", user)
		session.Options(sessions.Options{
			MaxAge:   172800, // Expire session every 2 days
			Secure:   true,
			HttpOnly: true,
		})

		if err := session.Save(); err != nil {
			response.Error(ctx, apierrors.NewInternalError("Couldn't save the session"))
			ctx.Abort()
			return
		}
	}
}

// tokenAuthMiddleware is only used to establish websocket connections for authenticated users
func tokenAuthMiddleware(ctx *gin.Context) {
	logger := requestctx.Logger(ctx.Request.Context()).WithName("TokenAuthMiddleware")
	logger.V(1).Info("Authtoken authentication")

	token := ctx.Query("authtoken")
	claims, err := authtoken.Validate(token)
	if err != nil {
		apiErr := apierrors.NewAPIError("unknown token validation error", "", http.StatusUnauthorized)
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
