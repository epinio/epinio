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

package dex

import (
	"crypto/sha256"
	"encoding/base64"

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

	return base64.RawURLEncoding.EncodeToString(hash)
}
