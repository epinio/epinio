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
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/dex"
	"github.com/epinio/epinio/internal/helmchart"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// Authentication middleware authenticates the user either using the basic auth or the bearer token (OIDC)
func Authentication(ctx *gin.Context) {
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

	// Bail early if the request has no proper credentials embedded into it.
	username, password, ok := ctx.Request.BasicAuth()
	if !ok {
		return auth.User{}, apierrors.NewInternalError("Couldn't extract user or password from the auth header")
	}

	userMap, err := loadUsersMap(ctx, logger)
	if err != nil {
		return auth.User{}, apierrors.InternalError(err)
	}

	if len(userMap) == 0 {
		return auth.User{}, apierrors.NewAPIError("no user found", http.StatusUnauthorized)
	}

	err = bcrypt.CompareHashAndPassword([]byte(userMap[username].Password), []byte(password))
	if err != nil {
		return auth.User{}, apierrors.NewAPIError("wrong user or password", http.StatusUnauthorized)
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

	user, err := getOrCreateUserByEmail(ctx, logger, claims.Email, role)
	if err != nil {
		return auth.User{}, apierrors.InternalError(err, "getting/creating user with email")
	}

	logger.V(1).Info("token verified", "user", fmt.Sprintf("%#v", user))

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

func loadUsersMap(ctx context.Context, logger logr.Logger) (map[string]auth.User, error) {
	authService, err := auth.NewAuthServiceFromContext(ctx, logger)
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

func getOrCreateUserByEmail(ctx context.Context, logger logr.Logger, email, role string) (auth.User, error) {
	user := auth.User{}
	var err error

	authService, err := auth.NewAuthServiceFromContext(ctx, logger)
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
