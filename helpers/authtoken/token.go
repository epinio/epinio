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

// Package authtoken creates JWT tokens to secure the websockets connections
package authtoken

import (
	"crypto/rand"
	"crypto/rsa"
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
	MaxExpiry = 30 * time.Second

	// DefaultExpiry for the auth token
	DefaultExpiry = MaxExpiry
)

// EpinioClaims are the values we store in the JWT
type EpinioClaims struct {
	jwt.RegisteredClaims
	Username string `json:"user"`
}

func init() {
	// generate ephemeral keys
	var err error
	privKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("cannot generate key")
	}
	pubKey = &privKey.PublicKey
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
