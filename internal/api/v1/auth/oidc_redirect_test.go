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
	"net/http/httptest"
	"os"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("oidcCallbackURL", func() {
	AfterEach(func() {
		_ = os.Unsetenv(oidcRedirectURIEnv)
	})

	It("uses EPINIO_OIDC_REDIRECT_URI when set", func() {
		Expect(os.Setenv(oidcRedirectURIEnv, "https://custom.example/cb")).To(Succeed())

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, err := http.NewRequest(http.MethodGet, "https://epinio.example/api/v1/auth/oidc/start", nil)
		Expect(err).ToNot(HaveOccurred())
		c.Request = req

		Expect(oidcCallbackURL(c)).To(Equal("https://custom.example/cb"))
	})

	It("builds URL from request host and forwarded proto", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, err := http.NewRequest(http.MethodGet, "http://epinio.example/api/v1/auth/oidc/start", nil)
		Expect(err).ToNot(HaveOccurred())
		req.Host = "epinio.example"
		req.Header.Set("X-Forwarded-Proto", "https")
		c.Request = req

		Expect(oidcCallbackURL(c)).To(Equal("https://epinio.example/api/v1/auth/oidc/callback"))
	})

	It("prefers X-Forwarded-Host", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, err := http.NewRequest(http.MethodGet, "https://internal/api/v1/auth/oidc/start", nil)
		Expect(err).ToNot(HaveOccurred())
		req.Host = "internal"
		req.Header.Set("X-Forwarded-Host", "epinio.public.example")
		c.Request = req

		Expect(oidcCallbackURL(c)).To(Equal("https://epinio.public.example/api/v1/auth/oidc/callback"))
	})
})
