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
	maxExpiry = 30 * time.Second

	// DefaultExpiry for the auth token
	DefaultExpiry = maxExpiry
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
	if s > maxExpiry {
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
