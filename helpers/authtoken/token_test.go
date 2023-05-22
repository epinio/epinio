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

package authtoken_test

import (
	"time"

	"github.com/epinio/epinio/helpers/authtoken"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Token", func() {
	It("returns a valid JWT token and accepts it", func() {
		token := authtoken.Create("armin", authtoken.DefaultExpiry)
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
})
