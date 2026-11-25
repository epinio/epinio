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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/epinio/epinio/internal/auth/session"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func perform(method, path string, body []byte, setup func(*gin.Context)) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Handle(method, path, func(ctx *gin.Context) {
		if setup != nil {
			setup(ctx)
		}
		switch path {
		case "/api/v1/auth/login":
			Login(ctx)
		case "/api/v1/auth/logout":
			Logout(ctx)
		case "/api/v1/auth/oidc/callback":
			OIDCCallback(ctx)
		}
	})

	req, err := http.NewRequest(method, path, bytes.NewReader(body))
	Expect(err).ToNot(HaveOccurred())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	engine.ServeHTTP(w, req)
	return w
}

var _ = Describe("Auth handlers", func() {
	Describe("POST /api/v1/auth/login", func() {
		It("returns 400 when body is empty object", func() {
			w := perform(http.MethodPost, "/api/v1/auth/login", []byte(`{}`), nil)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 400 when username is missing", func() {
			w := perform(http.MethodPost, "/api/v1/auth/login", []byte(`{"password":"x"}`), nil)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 400 when password is missing", func() {
			w := perform(http.MethodPost, "/api/v1/auth/login", []byte(`{"username":"admin"}`), nil)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("POST /api/v1/auth/logout", func() {
		It("returns 200 and clears auth cookies", func() {
			w := perform(http.MethodPost, "/api/v1/auth/logout", []byte(`{}`), func(c *gin.Context) {
				Expect(session.SetSessionCookies(c, "admin")).To(Succeed())
			})

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]string
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["status"]).To(Equal("ok"))

			cleared := map[string]bool{}
			for _, cookie := range w.Result().Cookies() {
				if cookie.MaxAge < 0 || cookie.Value == "" {
					cleared[cookie.Name] = true
				}
			}
			Expect(cleared).To(HaveKey(session.SessionCookie))
			Expect(cleared).To(HaveKey(session.CSRFCookie))
		})
	})

	Describe("GET /api/v1/auth/oidc/callback", func() {
		It("returns 401 when provider returns error query", func() {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)
			engine.GET("/api/v1/auth/oidc/callback", OIDCCallback)

			req, err := http.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?error=access_denied&error_description=nope", nil)
			Expect(err).ToNot(HaveOccurred())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})

		It("returns 400 when code or state is missing", func() {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)
			engine.GET("/api/v1/auth/oidc/callback", OIDCCallback)

			req, err := http.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?code=abc", nil)
			Expect(err).ToNot(HaveOccurred())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 401 when OIDC flow cookie is missing", func() {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)
			engine.GET("/api/v1/auth/oidc/callback", OIDCCallback)

			req, err := http.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?code=abc&state=xyz", nil)
			Expect(err).ToNot(HaveOccurred())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})

		It("returns 401 on state mismatch", func() {
			gin.SetMode(gin.TestMode)
			wSetup := httptest.NewRecorder()
			cSetup, _ := gin.CreateTestContext(wSetup)
			reqSetup, err := http.NewRequest(http.MethodGet, "/", nil)
			Expect(err).ToNot(HaveOccurred())
			cSetup.Request = reqSetup
			Expect(session.SetOIDCFlowCookie(cSetup, "verifier", "expected-state")).To(Succeed())

			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)
			engine.GET("/api/v1/auth/oidc/callback", OIDCCallback)

			req, err := http.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?code=abc&state=wrong-state", nil)
			Expect(err).ToNot(HaveOccurred())
			for _, cookie := range wSetup.Result().Cookies() {
				req.AddCookie(cookie)
			}
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
			Expect(w.Body.String()).To(ContainSubstring("state mismatch"))
		})
	})
})
