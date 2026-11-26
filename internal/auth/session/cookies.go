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

// Package session provides browser session and CSRF cookie helpers for the
// Epinio API server (EPINIO-837).
package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/authtoken"
	"github.com/gin-gonic/gin"
)

const (
	// SessionCookie is the HttpOnly cookie holding the signed session JWT.
	SessionCookie = "EPINIO_SESSION"

	// CSRFCookie is readable by JS so the UI can send x-api-csrf on mutating
	// requests. Named EPINIO_CSRF so it does not clash with Jetstream's CSRF.
	CSRFCookie = "EPINIO_CSRF"

	// OIDCFlowCookie holds the short-lived signed PKCE verifier + state.
	OIDCFlowCookie = "EPINIO_OIDC_FLOW"

	// CSRFHeader is the request header the UI sends on non-GET requests.
	CSRFHeader = "x-api-csrf"
)

// SetSessionCookies writes the session JWT and matching CSRF cookie.
func SetSessionCookies(c *gin.Context, username string) error {
	csrf, err := randomToken(32)
	if err != nil {
		return err
	}

	token := authtoken.CreateSession(username, csrf, authtoken.GetSessionExpiry())
	if token == "" {
		return errFailedSession
	}

	maxAge := int(authtoken.GetSessionExpiry().Seconds())
	setCookie(c, SessionCookie, token, maxAge, true)
	setCookie(c, CSRFCookie, csrf, maxAge, false)
	return nil
}

// ClearCookies clears session, CSRF, and OIDC flow cookies.
func ClearCookies(c *gin.Context) {
	clearCookie(c, SessionCookie)
	clearCookie(c, CSRFCookie)
	clearCookie(c, OIDCFlowCookie)
}

// SetOIDCFlowCookie stores PKCE verifier + state + nonce for the OIDC callback.
func SetOIDCFlowCookie(c *gin.Context, codeVerifier, state, nonce string) error {
	token := authtoken.CreateOIDCFlow(codeVerifier, state, nonce, authtoken.MaxOIDCFlowExpiry)
	if token == "" {
		return errFailedSession
	}
	setCookie(c, OIDCFlowCookie, token, int(authtoken.MaxOIDCFlowExpiry.Seconds()), true)
	return nil
}

// ClearOIDCFlowCookie removes the short-lived OIDC PKCE cookie.
func ClearOIDCFlowCookie(c *gin.Context) {
	clearCookie(c, OIDCFlowCookie)
}

// ReadOIDCFlow returns verifier, state, and nonce from the OIDC flow cookie.
func ReadOIDCFlow(c *gin.Context) (codeVerifier, state, nonce string, err error) {
	raw, err := c.Cookie(OIDCFlowCookie)
	if err != nil {
		return "", "", "", err
	}
	claims, err := authtoken.ValidateOIDCFlow(raw)
	if err != nil {
		return "", "", "", err
	}
	return claims.CodeVerifier, claims.State, claims.Nonce, nil
}

// AuthenticateCookie validates the session cookie. For non-safe methods it also
// requires a CSRF header that matches the session.
func AuthenticateCookie(c *gin.Context) (username string, err error) {
	raw, err := c.Cookie(SessionCookie)
	if err != nil {
		return "", err
	}

	claims, err := authtoken.ValidateSession(raw)
	if err != nil {
		return "", err
	}

	if !isSafeMethod(c.Request.Method) {
		header := c.GetHeader(CSRFHeader)
		if header == "" {
			header = c.GetHeader("X-Api-Csrf")
		}
		if !csrfMatches(claims.CSRF, header) {
			return "", errCSRFMismatch
		}
	}

	return claims.Username, nil
}

func csrfMatches(expected, header string) bool {
	if expected == "" || header == "" {
		return false
	}
	// UI historically appends a trailing "=" when forwarding the cookie value.
	header = strings.TrimSuffix(header, "=")
	if subtle.ConstantTimeCompare([]byte(header), []byte(expected)) == 1 {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(header+"="), []byte(expected)) == 1
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func sessionSameSite() http.SameSite {
	// Default Lax for same-origin deployments. Set EPINIO_SESSION_SAMESITE=none
	// when Rancher-extension (cross-origin) credentialed cookie auth is required.
	switch strings.ToLower(strings.TrimSpace(os.Getenv("EPINIO_SESSION_SAMESITE"))) {
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func setCookie(c *gin.Context, name, value string, maxAge int, httpOnly bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: httpOnly,
		Secure:   true,
		SameSite: sessionSameSite(),
	})
}

func clearCookie(c *gin.Context, name string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   true,
		SameSite: sessionSameSite(),
	})
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type sessionError string

func (e sessionError) Error() string { return string(e) }

const (
	errFailedSession sessionError = "failed to create session token"
	errCSRFMismatch  sessionError = "csrf token mismatch"
)
