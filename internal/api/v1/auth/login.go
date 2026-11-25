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

// Package auth implements browser login/logout and OIDC start/callback
// endpoints that issue session cookies (EPINIO-837).
package auth

import (
	"net/http"

	"github.com/epinio/epinio/internal/api/v1/response"
	epinioauth "github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/auth/session"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

// Login handles POST /api/v1/auth/login — bcrypt check, then session + CSRF cookies.
func Login(c *gin.Context) {
	logger := requestctx.Logger(c.Request.Context()).With("component", "auth-login")

	var req loginRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, apierrors.NewBadRequestError(err.Error()).WithDetails("username and password required"))
		return
	}
	if req.Username == "" || req.Password == "" {
		response.Error(c, apierrors.NewBadRequestError("username and password required"))
		return
	}

	authService, err := epinioauth.NewAuthServiceFromContext(c)
	if err != nil {
		response.Error(c, apierrors.InternalError(err, "couldn't create auth service from context"))
		return
	}

	user, err := authService.GetUserByUsername(c, req.Username)
	if err != nil {
		logger.Debugw("login failed", "reason", "user lookup", "err", err)
		response.Error(c, apierrors.NewAPIError("wrong user or password", http.StatusUnauthorized))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		logger.Debugw("login failed", "reason", "password mismatch")
		response.Error(c, apierrors.NewAPIError("wrong user or password", http.StatusUnauthorized))
		return
	}

	if err := session.SetSessionCookies(c, user.Username); err != nil {
		response.Error(c, apierrors.InternalError(err, "setting session cookies"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user.Username,
	})
}

// Logout handles POST /api/v1/auth/logout — clears session/CSRF/OIDC cookies.
// Sessions are signed JWTs (no server-side store), so clearing cookies is revocation.
func Logout(c *gin.Context) {
	session.ClearCookies(c)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
