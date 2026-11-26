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

// Package authtoken creates JWT tokens to secure the websockets connections
// and browser sessions. Keys are loaded from AUTH_TOKEN_PRIVATE_KEY /
// AUTH_TOKEN_PRIVATE_KEY_FILE when set so replicas share the same signer;
// otherwise an ephemeral key is generated (local/dev).
package authtoken

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// we could switch to HMAC, no verification is required by the client.
var (
	pubKey  *rsa.PublicKey
	privKey *rsa.PrivateKey
	alg     = jwt.SigningMethodRS384
)

const (
	// MaxExpiry is the maximum allowed expiry time for websocket tokens
	MaxExpiry = 5 * time.Minute

	// DefaultExpiry for the auth token when not configured
	DefaultExpiry = 30 * time.Second

	// AuthTokenDefaultExpiryEnv is the environment variable for configuring default token expiry
	AuthTokenDefaultExpiryEnv = "AUTH_TOKEN_DEFAULT_EXPIRY" // nolint:gosec

	// PrivateKeyEnv holds a PEM-encoded RSA private key (shared across replicas)
	PrivateKeyEnv = "AUTH_TOKEN_PRIVATE_KEY" // nolint:gosec

	// PrivateKeyFileEnv is a path to a PEM-encoded RSA private key file
	PrivateKeyFileEnv = "AUTH_TOKEN_PRIVATE_KEY_FILE" // nolint:gosec

	// MaxSessionExpiry caps browser session JWT lifetime
	MaxSessionExpiry = 24 * time.Hour

	// DefaultSessionExpiry for browser session cookies
	DefaultSessionExpiry = 24 * time.Hour

	// SessionExpiryEnv configures browser session lifetime (e.g. "12h", "24h")
	SessionExpiryEnv = "SESSION_COOKIE_EXPIRY"

	// MaxOIDCFlowExpiry caps the short-lived OIDC PKCE cookie
	MaxOIDCFlowExpiry = 10 * time.Minute

	// KindSession marks a browser session JWT
	KindSession = "session"

	// KindOIDCFlow marks a short-lived OIDC start cookie JWT
	KindOIDCFlow = "oidc-flow"
)

// GetDefaultExpiry returns the configured default token expiry from AUTH_TOKEN_DEFAULT_EXPIRY
// (e.g. "30s", "60s", "2m"), or DefaultExpiry if not set or invalid.
// Values are capped at MaxExpiry for security.
func GetDefaultExpiry() time.Duration {
	if val := os.Getenv(AuthTokenDefaultExpiryEnv); val != "" {
		if d, err := time.ParseDuration(val); err == nil && d > 0 {
			if d > MaxExpiry {
				return MaxExpiry
			}
			return d
		}
	}
	return DefaultExpiry
}

// GetSessionExpiry returns the browser session cookie lifetime.
func GetSessionExpiry() time.Duration {
	if val := os.Getenv(SessionExpiryEnv); val != "" {
		if d, err := time.ParseDuration(val); err == nil && d > 0 {
			if d > MaxSessionExpiry {
				return MaxSessionExpiry
			}
			return d
		}
	}
	return DefaultSessionExpiry
}

// EpinioClaims are the values we store in the JWT
type EpinioClaims struct {
	jwt.RegisteredClaims
	Username string `json:"user"`
	CSRF     string `json:"csrf,omitempty"`
	Kind     string `json:"kind,omitempty"`
	// OIDC PKCE flow fields (KindOIDCFlow only)
	CodeVerifier string `json:"code_verifier,omitempty"`
	State        string `json:"state,omitempty"`
	Nonce        string `json:"nonce,omitempty"`
}

func init() {
	if err := loadKeys(); err != nil {
		panic("cannot load or generate auth token key: " + err.Error())
	}
}

func loadKeys() error {
	if pemStr := os.Getenv(PrivateKeyEnv); pemStr != "" {
		return setPrivateKeyPEM([]byte(pemStr))
	}
	if path := os.Getenv(PrivateKeyFileEnv); path != "" {
		pemBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		return setPrivateKeyPEM(pemBytes)
	}

	var err error
	privKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	pubKey = &privKey.PublicKey
	return nil
}

func setPrivateKeyPEM(pemBytes []byte) error {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return fmt.Errorf("no PEM block found in auth token private key")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		privKey = key
		pubKey = &privKey.PublicKey
		return nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parsing auth token private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("auth token private key is not RSA")
	}
	privKey = key
	pubKey = &privKey.PublicKey
	return nil
}

// Create a new token, that uses a short lifetime, think one request.
// WARNING: It should only be used to establish the websocket connection once,
// because we can't revoke and don't check for deleted users.
func Create(user string, s time.Duration) string {
	// seriously, don't use a long expiry time with this code
	if s > MaxExpiry {
		return ""
	}
	claims := EpinioClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s)),
			Issuer:    "epinio-server",
		},
		Username: user,
	}

	return sign(claims)
}

// CreateSession creates a longer-lived browser session JWT bound to a CSRF token.
func CreateSession(user, csrf string, s time.Duration) string {
	if s <= 0 || s > MaxSessionExpiry || user == "" || csrf == "" {
		return ""
	}
	claims := EpinioClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s)),
			Issuer:    "epinio-server",
		},
		Username: user,
		CSRF:     csrf,
		Kind:     KindSession,
	}
	return sign(claims)
}

// CreateOIDCFlow creates a short-lived JWT storing PKCE verifier + OAuth state + nonce.
func CreateOIDCFlow(codeVerifier, state, nonce string, s time.Duration) string {
	if s <= 0 || s > MaxOIDCFlowExpiry || codeVerifier == "" || state == "" || nonce == "" {
		return ""
	}
	claims := EpinioClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s)),
			Issuer:    "epinio-server",
		},
		Kind:         KindOIDCFlow,
		CodeVerifier: codeVerifier,
		State:        state,
		Nonce:        nonce,
	}
	return sign(claims)
}

func sign(claims EpinioClaims) string {
	token := jwt.NewWithClaims(alg, claims)
	str, err := token.SignedString(privKey)
	if err != nil {
		return ""
	}
	return str
}

// Validate makes sure the token is created by and not expired
func Validate(t string) (*EpinioClaims, error) {
	token, err := jwt.ParseWithClaims(
		t,
		&EpinioClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return pubKey, nil
		},
		// we don't publish the public key, but just to be safe, make
		// sure we only support rsa
		jwt.WithValidMethods([]string{alg.Name}),
	)
	if err != nil {
		return nil, err
	}

	claims := token.Claims.(*EpinioClaims)
	return claims, nil
}

// ValidateSession validates a browser session JWT.
func ValidateSession(t string) (*EpinioClaims, error) {
	claims, err := Validate(t)
	if err != nil {
		return nil, err
	}
	if claims.Kind != KindSession || claims.Username == "" || claims.CSRF == "" {
		return nil, fmt.Errorf("not a session token")
	}
	return claims, nil
}

// ValidateOIDCFlow validates a short-lived OIDC PKCE cookie JWT.
func ValidateOIDCFlow(t string) (*EpinioClaims, error) {
	claims, err := Validate(t)
	if err != nil {
		return nil, err
	}
	if claims.Kind != KindOIDCFlow || claims.CodeVerifier == "" || claims.State == "" || claims.Nonce == "" {
		return nil, fmt.Errorf("not an oidc flow token")
	}
	return claims, nil
}
