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
	"github.com/epinio/epinio/helpers/kubernetes"
	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/namespace"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/dex"
	"github.com/epinio/epinio/internal/domain"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/epinio/epinio/internal/version"
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

	// Dex or no dex ?

	dexPEMPath := "/etc/ssl/certs/dex-tls.pem"

	if _, err := os.Stat(dexPEMPath); err == nil {
		// dex secret is present, load contained cert

		err := auth.ExtendLocalTrustFromFile(dexPEMPath)
		if err != nil {
			return nil, errors.Wrap(err, "extending local trust with dex")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		// No dex secret/file, do nothing
	} else {
		// Some other Stat error, report
		return nil, errors.Wrap(err, "extending local trust with dex")
	}

	// init routes

	// TODO not sure why it needs a context (it's used in the Platform)
	cluster, err := kubernetes.GetCluster(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "cannot create Kubernetes Client")
	}

	// create the needed controllers
	namespaceController := namespace.NewController(namespaces.NewKubernetesService(cluster))

	// setup routes
	apiv1.Routes.SetRoutes(apiv1.MakeRoutes()...)
	apiv1.Routes.SetRoutes(apiv1.MakeNamespaceRoutes(namespaceController)...)
	apiv1.Routes.SetRoutes(apiv1.MakeWsRoutes()...)

	// Register api routes
	{
		apiRoutesGroup := router.Group(apiv1.Root,
			authMiddleware,
			versionMiddleware,
			apiv1.NamespaceMiddleware,
			apiv1.AuthorizationMiddleware,
		)
		apiv1.Lemon(apiRoutesGroup)
	}

	// Register web socket routes
	{
		wapiRoutesGroup := router.Group(apiv1.WsRoot,
			tokenAuthMiddleware,
			versionMiddleware,
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

var oidcProvider *dex.OIDCProvider

// getOIDCProvider returns a lazy constructed OIDC provider
func getOIDCProvider(ctx context.Context) (*dex.OIDCProvider, error) {
	if oidcProvider != nil {
		return oidcProvider, nil
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get access to a kube client")
	}

	secret, err := cluster.GetSecret(ctx, helmchart.Namespace(), "dex-config")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dex-config secret")
	}

	config, err := dex.NewConfigFromSecretData("epinio-api", secret.Data)
	if err != nil {
		return nil, errors.Wrap(err, "parsing the issuer URL")
	}

	oidcProvider, err := dex.NewOIDCProviderWithConfig(ctx, config)
	if err != nil {
		return nil, errors.Wrap(err, "constructing dexProviderConfig")
	}

	return oidcProvider, nil
}

func versionMiddleware(ctx *gin.Context) {
	ctx.Header(apiv1.VersionHeader, version.Version)
}

// authMiddleware authenticates the user either using the basic auth or the bearer token (OIDC)
func authMiddleware(ctx *gin.Context) {
	// we need this check to return a 401 instead of an error
	authorizationHeader := ctx.Request.Header.Get("Authorization")
	if authorizationHeader == "" {
		response.Error(ctx, apierrors.NewAPIError("missing credentials", http.StatusUnauthorized))
		ctx.Abort()
		return
	}

	var user auth.User
	var authError apierrors.APIErrors

	if strings.HasPrefix(authorizationHeader, "Basic ") {
		user, authError = basicAuthentication(ctx)
	} else if strings.HasPrefix(authorizationHeader, "Bearer ") {
		user, authError = oidcAuthentication(ctx)
	} else {
		authError = apierrors.NewAPIError("not supported Authorization Header", http.StatusUnauthorized)
	}

	if authError != nil {
		response.Error(ctx, authError)
		ctx.Abort()
		return
	}

	// Write the user info in the context. It's needed by the next middleware
	// to write it into the session.
	newCtx := ctx.Request.Context()
	newCtx = requestctx.WithUser(newCtx, user)
	ctx.Request = ctx.Request.Clone(newCtx)
}

// basicAuthentication performs the Basic Authentication
func basicAuthentication(ctx *gin.Context) (auth.User, apierrors.APIErrors) {
	reqCtx := ctx.Request.Context()
	logger := requestctx.Logger(reqCtx).WithName("basicAuthentication")
	logger.V(1).Info("starting Basic Authentication")

	userMap, err := loadUsersMap(ctx)
	if err != nil {
		return auth.User{}, apierrors.InternalError(err)
	}

	if len(userMap) == 0 {
		return auth.User{}, apierrors.NewAPIError("no user found", http.StatusUnauthorized)
	}

	username, password, ok := ctx.Request.BasicAuth()
	if !ok {
		return auth.User{}, apierrors.NewInternalError("Couldn't extract user from the auth header")
	}

	err = bcrypt.CompareHashAndPassword([]byte(userMap[username].Password), []byte(password))
	if err != nil {
		return auth.User{}, apierrors.NewAPIError("wrong password", http.StatusUnauthorized)
	}

	return userMap[username], nil
}

// oidcAuthentication perform the OIDC authentication with dex
func oidcAuthentication(ctx *gin.Context) (auth.User, apierrors.APIErrors) {
	reqCtx := ctx.Request.Context()
	logger := requestctx.Logger(reqCtx).WithName("oidcAuthentication")
	logger.V(1).Info("starting OIDC Authentication")

	oidcProvider, err := getOIDCProvider(ctx)
	if err != nil {
		return auth.User{}, apierrors.InternalError(err, "error getting OIDC provider")
	}

	authHeader := ctx.Request.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	idToken, err := oidcProvider.Verify(ctx, token)
	if err != nil {
		return auth.User{}, apierrors.NewAPIError(errors.Wrap(err, "token verification failed").Error(), http.StatusUnauthorized)
	}

	var claims struct {
		Email           string   `json:"email"`
		Groups          []string `json:"groups"`
		FederatedClaims struct {
			ConnectorID string `json:"connector_id"`
		} `json:"federated_claims"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return auth.User{}, apierrors.NewAPIError(errors.Wrap(err, "error parsing claims").Error(), http.StatusUnauthorized)
	}

	role := getRoleFromProviderGroups(logger, oidcProvider, claims.FederatedClaims.ConnectorID, claims.Groups)

	user, err := getOrCreateUserByEmail(ctx, claims.Email, role)
	if err != nil {
		return auth.User{}, apierrors.InternalError(err, "getting/creating user with email")
	}

	logger.V(1).Info("token verified", "user", fmt.Sprintf("%#v", user))

	return user, nil
}

// getRoleFromProviderGroups returns the user role, looking for it in the groups defined for the provider.
// If there are no groups that matches then the default role 'user' is returned.
// If a user has more than one group matching, the first from the Dex Configuration will be returned.
func getRoleFromProviderGroups(logger logr.Logger, oidcProvider *dex.OIDCProvider, providerID string, groups []string) string {
	defaultRole := "user"

	pg, err := oidcProvider.GetProviderGroups(providerID)
	if err != nil {
		logger.Info(
			"error getting provider groups",
			"provider", providerID,
		)

		return defaultRole
	}

	roles := pg.GetRolesFromGroups(groups...)
	if len(roles) == 0 {
		logger.Info(
			"no matching groups found in provider groups",
			"provider", providerID,
			"providerGroups", pg,
			"groups", strings.Join(groups, ","),
		)

		return defaultRole
	}

	return roles[0]
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

func getOrCreateUserByEmail(ctx context.Context, email, role string) (auth.User, error) {
	user := auth.User{}
	var err error

	authService, err := auth.NewAuthServiceFromContext(ctx)
	if err != nil {
		return user, errors.Wrap(err, "couldn't create auth service from context")
	}

	user, err = authService.GetUserByUsername(ctx, email)
	if err != nil {
		if err != auth.ErrUserNotFound {
			return user, errors.Wrap(err, "couldn't get user")
		}

		user.Username = email
		user.Role = role
		user, err = authService.SaveUser(ctx, user)
		if err != auth.ErrUserNotFound {
			return user, errors.Wrap(err, "couldn't create user")
		}
	}

	return user, nil
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
