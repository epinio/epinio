package dex

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"github.com/dchest/uniuri"
)

// CodeVerifier is an helper struct used to create a code_challenge for the PKCE
// Ref: https://www.oauth.com/oauth2-servers/pkce/
type CodeVerifier struct {
	Value string
}

// NewCodeVerifier returns a cryptographic secure random CodeVerifier of a fixed length (32)
func NewCodeVerifier() *CodeVerifier {
	return NewCodeVerifierWithLen(32)
}

// NewCodeVerifier returns a cryptographic secure random CodeVerifier of the specified length
func NewCodeVerifierWithLen(len int) *CodeVerifier {
	return &CodeVerifier{Value: uniuri.NewLen(len)}
}

// ChallengeS256 returns an encoded SHA256 code_challenge of the code_verifier
func (c *CodeVerifier) ChallengeS256() string {
	h := sha256.New()
	h.Write([]byte(c.Value))
	hash := h.Sum(nil)

	encoded := base64.StdEncoding.EncodeToString(hash)
	encoded = strings.Replace(encoded, "+", "-", -1)
	encoded = strings.Replace(encoded, "/", "_", -1)
	encoded = strings.Replace(encoded, "=", "", -1)

	return encoded
}
