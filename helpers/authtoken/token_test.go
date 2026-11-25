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

package authtoken_test

import (
	"os"
	"time"

	"github.com/epinio/epinio/helpers/authtoken"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Token", func() {
	It("returns a valid JWT token and accepts it", func() {
		token := authtoken.Create("armin", authtoken.GetDefaultExpiry())
		Expect(token).ToNot(BeEmpty())

		claims, err := authtoken.Validate(token)
		Expect(err).ToNot(HaveOccurred())
		Expect(claims.ExpiresAt).ToNot(BeNil())
		Expect(claims.Username).To(Equal("armin"))
	})

	When("parsing an invalid token", func() {
		invalidType := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJsb2dnZWRJbkFzIjoiYWRtaW4iLCJpYXQiOjE0MjI3Nzk2Mzh9.gzSraSYS8EXBxLN_oWnFSRgCzcmJmMjLiuyu5CSpyHI`
		invalidKey := `eyJhbGciOiJSUzM4NCIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJlcGluaW8tc2VydmVyIiwiZXhwIjoxNjQzMjE5Njc5fQ.krQhtsstDZsSP6mq3GNSiUiKW-tkbVIfBDwckTr_3B6FLHD8CnbSzmm_3b3JHwOpUvOkFeIf6EE_iuEcX8-aoRF2fNRPfRokf026saxTHFzerPH2iHjqXQoItUs4isCIHpPZDZP2y8W9_x9WaACcHNFEx7vWWG26eep3uxOCvFI`

		It("fails for an invalid signing type", func() {
			_, err := authtoken.Validate(invalidType)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("signing method HS256 is invalid"))
		})

		It("fails for an invalid key", func() {
			_, err := authtoken.Validate(invalidKey)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("crypto/rsa: verification error"))
		})
	})

	It("fails for an expired token", func() {
		token := authtoken.Create("armin", 0*time.Second)
		Expect(token).ToNot(BeEmpty())

		_, err := authtoken.Validate(token)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("token is expired"))
	})

	It("does not create a token beyond the maximum allowed expiry time", func() {
		token := authtoken.Create("armin", authtoken.MaxExpiry+time.Second)
		Expect(token).To(BeEmpty())
	})

	It("creates and validates a browser session token with CSRF", func() {
		token := authtoken.CreateSession("armin", "csrf-token", authtoken.GetSessionExpiry())
		Expect(token).ToNot(BeEmpty())

		claims, err := authtoken.ValidateSession(token)
		Expect(err).ToNot(HaveOccurred())
		Expect(claims.Username).To(Equal("armin"))
		Expect(claims.CSRF).To(Equal("csrf-token"))
		Expect(claims.Kind).To(Equal(authtoken.KindSession))
	})

	It("rejects a websocket token as a session", func() {
		token := authtoken.Create("armin", authtoken.GetDefaultExpiry())
		Expect(token).ToNot(BeEmpty())

		_, err := authtoken.ValidateSession(token)
		Expect(err).To(HaveOccurred())
	})

	It("creates and validates an OIDC flow token", func() {
		token := authtoken.CreateOIDCFlow("verifier", "state-xyz", authtoken.MaxOIDCFlowExpiry)
		Expect(token).ToNot(BeEmpty())

		claims, err := authtoken.ValidateOIDCFlow(token)
		Expect(err).ToNot(HaveOccurred())
		Expect(claims.CodeVerifier).To(Equal("verifier"))
		Expect(claims.State).To(Equal("state-xyz"))
	})

	It("rejects CreateSession with empty user or csrf", func() {
		Expect(authtoken.CreateSession("", "csrf", authtoken.GetSessionExpiry())).To(BeEmpty())
		Expect(authtoken.CreateSession("armin", "", authtoken.GetSessionExpiry())).To(BeEmpty())
	})

	It("rejects CreateSession beyond MaxSessionExpiry", func() {
		Expect(authtoken.CreateSession("armin", "csrf", authtoken.MaxSessionExpiry+time.Hour)).To(BeEmpty())
	})

	It("rejects CreateOIDCFlow with empty fields or too-long expiry", func() {
		Expect(authtoken.CreateOIDCFlow("", "state", authtoken.MaxOIDCFlowExpiry)).To(BeEmpty())
		Expect(authtoken.CreateOIDCFlow("verifier", "", authtoken.MaxOIDCFlowExpiry)).To(BeEmpty())
		Expect(authtoken.CreateOIDCFlow("verifier", "state", authtoken.MaxOIDCFlowExpiry+time.Minute)).To(BeEmpty())
	})

	It("rejects a session token as an OIDC flow token", func() {
		token := authtoken.CreateSession("armin", "csrf", authtoken.GetSessionExpiry())
		Expect(token).ToNot(BeEmpty())

		_, err := authtoken.ValidateOIDCFlow(token)
		Expect(err).To(HaveOccurred())
	})

	It("honours SESSION_COOKIE_EXPIRY env within the max cap", func() {
		Expect(os.Setenv(authtoken.SessionExpiryEnv, "1h")).To(Succeed())
		DeferCleanup(func() { _ = os.Unsetenv(authtoken.SessionExpiryEnv) })

		Expect(authtoken.GetSessionExpiry()).To(Equal(time.Hour))
	})

	It("caps SESSION_COOKIE_EXPIRY at MaxSessionExpiry", func() {
		Expect(os.Setenv(authtoken.SessionExpiryEnv, "72h")).To(Succeed())
		DeferCleanup(func() { _ = os.Unsetenv(authtoken.SessionExpiryEnv) })

		Expect(authtoken.GetSessionExpiry()).To(Equal(authtoken.MaxSessionExpiry))
	})
})
