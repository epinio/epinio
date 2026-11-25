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

package auth

import (
	"net/http"
	"os"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	epinioauth "github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/auth/session"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/dex"
	"github.com/epinio/epinio/internal/helmchart"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const (
	uiClientID          = "epinio-ui"
	oidcRedirectURIEnv  = "EPINIO_OIDC_REDIRECT_URI"
	oidcRedirectPath    = "/api/v1/auth/oidc/callback"
)

// OIDCStart handles GET /api/v1/auth/oidc/start — Dex authorize URL + PKCE cookie, then redirect.
func OIDCStart(c *gin.Context) {
	logger := requestctx.Logger(c.Request.Context()).With("component", "auth-oidc-start")

	oidcProvider, err := getUIODCProvider(c)
	if err != nil {
		logger.Errorw("oidc provider", "error", err)
		response.Error(c, apierrors.InternalError(err, "oidc provider"))
		return
	}

	authCodeURL, codeVerifier, state := oidcProvider.AuthCodeURLWithPKCEAndState()
	if err := session.SetOIDCFlowCookie(c, codeVerifier, state); err != nil {
		response.Error(c, apierrors.InternalError(err, "setting oidc flow cookie"))
		return
	}

	c.Redirect(http.StatusFound, authCodeURL)
}

// OIDCCallback handles GET /api/v1/auth/oidc/callback — exchange code, provision user, set session, redirect /.
func OIDCCallback(c *gin.Context) {
	logger := requestctx.Logger(c.Request.Context()).With("component", "auth-oidc-callback")

	if errMsg := c.Query("error"); errMsg != "" {
		response.Error(c, apierrors.NewAPIError(errMsg+": "+c.Query("error_description"), http.StatusUnauthorized))
		return
	}

	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		response.Error(c, apierrors.NewBadRequestError("missing code or state"))
		return
	}

	codeVerifier, expectedState, err := session.ReadOIDCFlow(c)
	session.ClearOIDCFlowCookie(c)
	if err != nil {
		response.Error(c, apierrors.NewAPIError("invalid or expired oidc flow", http.StatusUnauthorized))
		return
	}
	if state != expectedState {
		response.Error(c, apierrors.NewAPIError("state mismatch", http.StatusUnauthorized))
		return
	}

	oidcProvider, err := getUIODCProvider(c)
	if err != nil {
		response.Error(c, apierrors.InternalError(err, "oidc provider"))
		return
	}

	token, err := oidcProvider.ExchangeWithPKCE(c.Request.Context(), code, codeVerifier)
	if err != nil {
		logger.Errorw("token exchange failed", "error", err)
		response.Error(c, apierrors.NewAPIError(errors.Wrap(err, "token exchange failed").Error(), http.StatusUnauthorized))
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		response.Error(c, apierrors.NewAPIError("id_token missing from token response", http.StatusUnauthorized))
		return
	}

	idToken, err := oidcProvider.Verify(c.Request.Context(), rawIDToken)
	if err != nil {
		response.Error(c, apierrors.NewAPIError(errors.Wrap(err, "token verification failed").Error(), http.StatusUnauthorized))
		return
	}

	var claims struct {
		Email           string   `json:"email"`
		Groups          []string `json:"groups"`
		FederatedClaims struct {
			ConnectorID string `json:"connector_id"`
		} `json:"federated_claims"`
	}
	if err := idToken.Claims(&claims); err != nil {
		response.Error(c, apierrors.NewAPIError(errors.Wrap(err, "error parsing claims").Error(), http.StatusUnauthorized))
		return
	}
	if claims.Email == "" {
		response.Error(c, apierrors.NewAPIError("email claim missing", http.StatusUnauthorized))
		return
	}

	roles := epinioauth.RolesFromProviderGroups(c.Request.Context(), oidcProvider, claims.FederatedClaims.ConnectorID, claims.Groups)
	user, err := epinioauth.GetOrCreateUserByEmail(c.Request.Context(), claims.Email, roles)
	if err != nil {
		response.Error(c, apierrors.InternalError(err, "getting/creating user with email"))
		return
	}

	updatedUser, needsUpdate := epinioauth.IsUpdateUserNeeded(user)
	if needsUpdate {
		authService, err := epinioauth.NewAuthServiceFromContext(c)
		if err != nil {
			response.Error(c, apierrors.InternalError(err, "couldn't create auth service from context"))
			return
		}
		user, err = authService.UpdateUser(c, updatedUser)
		if err != nil {
			response.Error(c, apierrors.InternalError(err, "updating user"))
			return
		}
	}

	if err := session.SetSessionCookies(c, user.Username); err != nil {
		response.Error(c, apierrors.InternalError(err, "setting session cookies"))
		return
	}

	c.Redirect(http.StatusFound, "/")
}

func getUIODCProvider(c *gin.Context) (*dex.OIDCProvider, error) {
	cluster, err := kubernetes.GetCluster(c.Request.Context())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get access to a kube client")
	}

	secret, err := cluster.GetSecret(c.Request.Context(), helmchart.Namespace(), "dex-config")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dex-config secret")
	}

	config, err := dex.NewConfigFromSecretData(uiClientID, secret.Data)
	if err != nil {
		return nil, errors.Wrap(err, "parsing dex config")
	}
	config.RedirectURL = oidcCallbackURL(c)

	oidcProvider, err := dex.NewOIDCProviderWithConfig(c.Request.Context(), config)
	if err != nil {
		return nil, errors.Wrap(err, "constructing oidc provider")
	}

	return oidcProvider, nil
}

func oidcCallbackURL(c *gin.Context) string {
	if v := strings.TrimSpace(os.Getenv(oidcRedirectURIEnv)); v != "" {
		return v
	}

	scheme := "https"
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	host := c.Request.Host
	if h := c.GetHeader("X-Forwarded-Host"); h != "" {
		host = h
	}

	return scheme + "://" + host + oidcRedirectPath
}
