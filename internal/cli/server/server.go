// Package server provides the Epinio http server
package server

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/epinio/epinio/helpers/termui"

	"github.com/epinio/epinio/helpers/tracelog"
	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/filesystem"
	"github.com/epinio/epinio/internal/web"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/mattn/go-colorable"
)

// Start is a helper which initializes and starts the API server
func Start(wg *sync.WaitGroup, port int, _ *termui.UI, logger logr.Logger) (*http.Server, string, error) {
	// Support colors on Windows also
	gin.DefaultWriter = colorable.NewColorableStdout()

	// Static files
	var assetsDir http.FileSystem
	if os.Getenv("LOCAL_FILESYSTEM") == "true" {
		assetsDir = http.Dir(path.Join(".", "assets", "embedded-web-files", "assets"))
	} else {
		assetsDir = filesystem.Assets()
	}

	listener, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		return nil, "", err
	}

	gin.SetMode(gin.ReleaseMode)

	// Endpoint structure ...
	// | Path              | Notes      | Logging
	// | ---               | ---        | ----
	// | <Root>/...        | API        | Via "<Root>" Group
	// | /ready            | L/R Probes |
	// | /assets           |            | Via "/assets" Group
	// | /                 | Dashboard  | Via individual attachment, web.Lemon()
	// | /info             | ditto      | ditto
	// | /orgs/target/:org | ditto      | ditto

	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(gin.Recovery())

	// No authentication, no logging, no session. This is the healthcheck.
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	if os.Getenv("SESSION_KEY") == "" {
		return nil, "", errors.New("SESSION_KEY environment variable not defined")
	}

	store := cookie.NewStore([]byte(os.Getenv("SESSION_KEY")))
	store.Options(sessions.Options{MaxAge: 60 * 60 * 24}) // expire in a day
	router.Use(sessions.Sessions("epinio-session", store))

	ginLogger := gin.LoggerWithFormatter(Formatter)
	authenticatedGroup := router.Group("", ginLogger, authMiddleware, sessionMiddleware)

	web.Lemon(authenticatedGroup)

	assets := authenticatedGroup.Group("/assets")
	assets.StaticFS("/", assetsDir)

	api := authenticatedGroup.Group(apiv1.Root)
	apiv1.Lemon(api)

	srv := &http.Server{
		Handler: router,
	}

	go func() {
		defer wg.Done() // let caller know we are done cleaning up

		// always returns error. ErrServerClosed on graceful close
		if err := srv.Serve(listener); err != http.ErrServerClosed {
			log.Fatalf("Epinio server failed to start: %v", err)
		}
	}()

	// report actual port back to user

	elements := strings.Split(listener.Addr().String(), ":")
	listeningPort := elements[len(elements)-1]

	return srv, listeningPort, nil
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
