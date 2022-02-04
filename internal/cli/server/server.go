// Package server provides the Epinio http server
package server

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/authtoken"
	"github.com/epinio/epinio/helpers/tracelog"
	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/golang-jwt/jwt/v4"
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
	router.Use(sessions.Sessions("epinio-session", store))

	ginLogger := gin.LoggerWithFormatter(Formatter)

	// Register routes
	// No authentication, no logging, no session. This is the healthcheck.
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	// Register web socket routes
	{
		rg := router.Group("", ginLogger, tokenAuthMiddleware)
		rg = rg.Group(apiv1.WsRoot)
		apiv1.Spice(rg)
	}

	// Register api routes
	{
		rg := router.Group("", ginLogger, authMiddleware, sessionMiddleware)
		rg = rg.Group(apiv1.Root)
		apiv1.Lemon(rg)
	}

	// print all registered routes
	if logger.V(15).Enabled() {
		for _, h := range router.Routes() {
			logger.V(15).Info(fmt.Sprintf("%-6s %-25s %s", h.Method, h.Path, h.Handler))
		}
	}

	return router, nil
}

func Formatter(params gin.LogFormatterParams) string {
	user := requestctx.User(params.Request.Context())
	return fmt.Sprintf("Method: %s | Path: %s | User: %s | IP: %s | Timestamp: %s | Latency: %s | Status: %d | Message: %s \n",
		params.Method, params.Path, user, params.ClientIP, params.TimeStamp.Format(time.RFC3339), params.Latency, params.StatusCode, params.ErrorMessage)
}

// authMiddleware authenticates the user either using the session or if one
// doesn't exist, it authenticates with basic auth.
func authMiddleware(ctx *gin.Context) {
	logger := tracelog.NewLogger().WithName("AuthMiddleware")

	// First get the available users
	accounts, err := auth.GetUserAccounts(ctx)
	if err != nil {
		response.Error(ctx, apierrors.InternalError(err))
	}

	if len(*accounts) == 0 {
		response.Error(ctx, apierrors.NewAPIError("no user found", "", http.StatusUnauthorized))
	}

	// We set this to the current user after successful authentication.
	// This is also added to the context for controllers to use.
	var user string

	session := sessions.Default(ctx)
	sessionUser := session.Get("user")
	if sessionUser == nil { // no session exists, try basic auth
		logger.V(1).Info("Basic auth authentication")
		authHeader := string(ctx.GetHeader("Authorization"))
		// If basic auth header is there, extract the user out of it
		if authHeader != "" {
			// A Basic auth header looks something like this:
			// Basic base64_encoded_username:password_string
			headerParts := strings.Split(authHeader, " ")
			if len(headerParts) < 2 {
				response.Error(ctx, apierrors.NewInternalError("Authorization header format was not expected"))
				ctx.Abort()
				return
			}
			creds, err := base64.StdEncoding.DecodeString(headerParts[1])
			if err != nil {
				response.Error(ctx, apierrors.NewInternalError("Couldn't decode auth header"))
				ctx.Abort()
				return
			}

			// creds is in username:password format
			user = strings.Split(string(creds), ":")[0]
			if user == "" {
				response.Error(ctx, apierrors.NewInternalError("Couldn't extract user from the auth header"))
				ctx.Abort()
				return
			}
		}

		// Perform basic auth authentication
		gin.BasicAuth(*accounts)(ctx)
	} else {
		logger.V(1).Info("Session authentication")
		var ok bool
		user, ok = sessionUser.(string)
		if !ok {
			response.Error(ctx, apierrors.NewInternalError("Couldn't parse user from session"))
			ctx.Abort()
			return
		}

		// Check if that user still exists. If not delete the session and block the request!
		// This allows us to kick out users even if they keep their browser open.
		userStillExists := false
		for checkUser := range *accounts {
			if checkUser == user {
				userStillExists = true
				break
			}
		}
		if !userStillExists {
			session.Clear()
			session.Options(sessions.Options{MaxAge: -1})
			err := session.Save()
			if err != nil {
				response.Error(ctx, apierrors.NewInternalError("Couldn't save the session"))
				ctx.Abort()
				return
			}
			response.Error(ctx, apierrors.NewAPIError("User no longer exists. Session expired.", "", http.StatusUnauthorized))
			ctx.Abort()
			return
		}
	}

	// Write the user info in the context. It's needed by the next middleware
	// to write it into the session.
	id := fmt.Sprintf("%d", rand.Intn(10000)) // nolint:gosec // Non-crypto use
	newCtx := ctx.Request.Context()
	newCtx = requestctx.ContextWithUser(newCtx, user)
	newCtx = requestctx.ContextWithID(newCtx, id)
	ctx.Request = ctx.Request.WithContext(newCtx)
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
	if user == "" { // This can't be, authentication has succeeded.
		response.Error(ctx, apierrors.NewInternalError("Couldn't set user in session after successful authentication. This can't happen."))
		ctx.Abort()
		return
	}
	if session.Get("user") == nil { // Only the first time after authentication success
		session.Set("user", user)
		session.Options(sessions.Options{
			MaxAge:   172800, // Expire session every 2 days
			Secure:   true,
			HttpOnly: true,
		})
		err := session.Save()
		if err != nil {
			response.Error(ctx, apierrors.NewInternalError("Couldn't save the session"))
			ctx.Abort()
			return
		}
	}
}

// tokenAuthMiddleware is only used to establish websocket connections for authenticated users
func tokenAuthMiddleware(ctx *gin.Context) {
	logger := tracelog.NewLogger().WithName("TokenAuthMiddleware")
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

	// we don't check if the user exists, token lifetime is small enough
	id := fmt.Sprintf("%d", rand.Intn(10000)) // nolint:gosec // Non-crypto use
	newCtx := ctx.Request.Context()
	newCtx = requestctx.ContextWithUser(newCtx, claims.Username)
	newCtx = requestctx.ContextWithID(newCtx, id)
	ctx.Request = ctx.Request.WithContext(newCtx)
}
