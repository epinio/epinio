// Copyright © 2021 - 2026 KRUMWARE LLC
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
	"context"
	"net/http"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/auth/session"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/dex"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"

	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Authentication middleware authenticates via Basic, Bearer (OIDC), or session cookie.
// Authorization header takes precedence; cookie + CSRF is used only when it is absent.
func Authentication(ctx *gin.Context) {
	authorizationHeader := ctx.Request.Header.Get("Authorization")

	authService, err := auth.NewAuthServiceFromContext(ctx)
	if err != nil {
		response.Error(ctx, apierrors.InternalError(err, "couldn't create auth service from context"))
		ctx.Abort()
		return
	}

	var user auth.User
	var authError apierrors.APIErrors

	switch {
	case authorizationHeader == "":
		user, authError = cookieAuthentication(ctx, authService)
	case strings.HasPrefix(authorizationHeader, "Basic "):
		user, authError = basicAuthentication(ctx, authService)
	case strings.HasPrefix(authorizationHeader, "Bearer "):
		user, authError = oidcAuthentication(ctx)
	default:
		authError = apierrors.NewAPIError("not supported Authorization Header", http.StatusUnauthorized)
	}

	if authError != nil {
		response.Error(ctx, authError)
		ctx.Abort()
		return
	}

	updatedUser, needsUpdate := auth.IsUpdateUserNeeded(user)
	if needsUpdate {
		user, err = authService.UpdateUser(ctx, updatedUser)
		if err != nil {
			response.Error(ctx, apierrors.InternalError(err, "updating user"))
			ctx.Abort()
			return
		}
	}

	// Write the user info in the context for downstream handlers/middleware.
	newCtx := ctx.Request.Context()
	newCtx = requestctx.WithUser(newCtx, user)
	ctx.Request = ctx.Request.Clone(newCtx)
}

// cookieAuthentication authenticates from EPINIO_SESSION (+ CSRF on mutating requests).
func cookieAuthentication(ctx *gin.Context, authService *auth.AuthService) (auth.User, apierrors.APIErrors) {
	username, err := session.AuthenticateCookie(ctx)
	if err != nil {
		return auth.User{}, apierrors.NewAPIError("missing credentials", http.StatusUnauthorized)
	}

	user, err := authService.GetUserByUsername(ctx, username)
	if err != nil {
		return auth.User{}, apierrors.NewAPIError(err.Error(), http.StatusUnauthorized)
	}

	if len(user.Roles) == 0 {
		if defaultRole, hasDefault := auth.EpinioRoles.Default(); hasDefault {
			user.Roles = auth.Roles{defaultRole}
		}
	}

	return user, nil
}

// basicAuthentication performs the Basic Authentication
func basicAuthentication(ctx *gin.Context, authService *auth.AuthService) (auth.User, apierrors.APIErrors) {
	logger := requestctx.Logger(ctx.Request.Context()).With("component", "basicAuthentication")
	logger.Debugw("starting Basic Authentication")

	// Bail early if the request has no proper credentials embedded into it.
	username, password, ok := ctx.Request.BasicAuth()
	if !ok {
		return auth.User{}, apierrors.NewInternalError("Couldn't extract user or password from the auth header")
	}

	user, err := authService.GetUserByUsername(ctx, username)
	if err != nil {
		return auth.User{}, apierrors.NewAPIError(err.Error(), http.StatusUnauthorized)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return auth.User{}, apierrors.NewAPIError("wrong user or password", http.StatusUnauthorized)
	}

	// if user has roles return
	if len(user.Roles) > 0 {
		return user, nil
	}

	// no roles found

	// if default is defined update the user with default role
	defaultRole, hasDefault := auth.EpinioRoles.Default()
	if hasDefault {
		user.Roles = auth.Roles{defaultRole}
	}

	return user, nil
}

// oidcAuthentication perform the OIDC authentication with dex
func oidcAuthentication(ctx *gin.Context) (auth.User, apierrors.APIErrors) {
	logger := requestctx.Logger(ctx.Request.Context()).With("component", "oidcAuthentication")
	logger.Debugw("starting OIDC Authentication")

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

	roles := auth.RolesFromProviderGroups(ctx.Request.Context(), oidcProvider, claims.FederatedClaims.ConnectorID, claims.Groups)

	user, err := auth.GetOrCreateUserByEmail(ctx, claims.Email, roles)
	if err != nil {
		return auth.User{}, apierrors.InternalError(err, "getting/creating user with email")
	}

	logger.Debugw("token verified", "user", user.Username)

	return user, nil
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

