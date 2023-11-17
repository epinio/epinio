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
	reqCtx := ctx.Request.Context()
	logger := requestctx.Logger(reqCtx).WithName("Authentication")

	// we need this check to return a 401 instead of an error
	authorizationHeader := ctx.Request.Header.Get("Authorization")
	if authorizationHeader == "" {
		response.Error(ctx, apierrors.NewAPIError("missing credentials", http.StatusUnauthorized))
		ctx.Abort()
		return
	}

	authService, err := auth.NewAuthServiceFromContext(ctx, logger)
	if err != nil {
		response.Error(ctx, apierrors.InternalError(err, "couldn't create auth service from context"))
		ctx.Abort()
		return
	}

	var user auth.User
	var authError apierrors.APIErrors

	if strings.HasPrefix(authorizationHeader, "Basic ") {
		user, authError = basicAuthentication(ctx, logger, authService)
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

	updatedUser, needsUpdate := auth.IsUpdateUserNeeded(logger, user)
	if needsUpdate {
		user, err = authService.UpdateUser(ctx, updatedUser)
		if err != nil {
			response.Error(ctx, apierrors.InternalError(err, "updating user"))
			ctx.Abort()
			return
		}
	}

	// Write the user info in the context. It's needed by the next middleware
	// to write it into the session.
	newCtx := ctx.Request.Context()
	newCtx = requestctx.WithUser(newCtx, user)
	ctx.Request = ctx.Request.Clone(newCtx)
}

// basicAuthentication performs the Basic Authentication
func basicAuthentication(ctx *gin.Context, logger logr.Logger, authService *auth.AuthService) (auth.User, apierrors.APIErrors) {
	logger = logger.WithName("basicAuthentication")
	logger.V(1).Info("starting Basic Authentication")

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

	roles := getRolesFromProviderGroups(logger, oidcProvider, claims.FederatedClaims.ConnectorID, claims.Groups)

	user, err := getOrCreateUserByEmail(ctx, logger, claims.Email, roles)
	if err != nil {
		return auth.User{}, apierrors.InternalError(err, "getting/creating user with email")
	}

	logger.V(1).Info("token verified", "user", user.Username)

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

// getRolesFromProviderGroups returns the user roles, looking for it in the groups defined for the provider.
func getRolesFromProviderGroups(logger logr.Logger, oidcProvider *dex.OIDCProvider, providerID string, groups []string) auth.Roles {
	roles := auth.Roles{}

	pg, err := oidcProvider.GetProviderGroups(providerID)
	if err != nil {
		logger.Info(
			"error getting provider groups",
			"provider", providerID,
		)

		return roles
	}

	roleIDs := pg.GetRolesFromGroups(groups...)
	if len(roleIDs) == 0 {
		logger.Info(
			"no matching groups found in provider groups",
			"provider", providerID,
			"providerGroups", pg,
			"groups", strings.Join(groups, ","),
		)

		return roles
	}

	for _, fullRoleID := range roleIDs {
		roleID, namespace := auth.ParseRoleID(fullRoleID)

		userRole, found := auth.EpinioRoles.FindByID(roleID)
		if !found {
			logger.Info(fmt.Sprintf("role not found in Epinio with roleID '%s'", roleID))
			continue
		}

		userRole.Namespace = namespace
		roles = append(roles, userRole)
	}

	return roles
}

// getOrCreateUserByEmail returns the user with the matching email, or if it not exists, it will create a new user.
// If some roles are provided then the user will be created or updated with those roles.
// If no roles are provided then we are going to check if a 'default' role was set. If so the user will be created
// or updated with the default role. The only exception to this behavior is if the user already exists and
// it had already some roles defined, maybe manually assigned.
// We don't want to clear/delete existing roles if no groups were provided.
func getOrCreateUserByEmail(ctx context.Context, logger logr.Logger, email string, userRoles auth.Roles) (auth.User, error) {
	user := auth.User{}
	var err error

	authService, err := auth.NewAuthServiceFromContext(ctx, logger)
	if err != nil {
		return user, errors.Wrap(err, "couldn't create auth service from context")
	}

	defaultRole, foundDefault := auth.EpinioRoles.Default()

	user, err = authService.GetUserByUsername(ctx, email)
	if err != nil {
		// something bad happened
		if err != auth.ErrUserNotFound {
			return user, errors.Wrap(err, "couldn't get user")
		}

		// user not found, create a new one
		user.Username = email

		// if no roles were found and a default was set create the user with the default
		if len(userRoles) == 0 && foundDefault {
			userRoles = append(userRoles, defaultRole)
		}

		user.Roles = userRoles
		user, err = authService.SaveUser(ctx, user)
		if err != nil {
			return user, errors.Wrap(err, "couldn't create user")
		}
		return user, nil
	}

	// no incoming roles where found
	if len(userRoles) == 0 {
		// the user already had some roles (maybe manually assigned)
		// we want to keep them without updating the user
		if len(user.Roles) > 0 {
			return user, nil
		}

		// if the user had no role we want to check if a default role was available
		if foundDefault {
			userRoles = append(userRoles, defaultRole)
		}
	}

	// update the roles of the existing user (with default, or the incoming roles)
	user.Roles = userRoles

	return user, nil
}
