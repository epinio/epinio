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

package session_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/epinio/epinio/helpers/authtoken"
	"github.com/epinio/epinio/internal/auth/session"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func newTestContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(method, path, nil)
	Expect(err).ToNot(HaveOccurred())
	c.Request = req
	return c, w
}

var _ = Describe("Session cookies", func() {
	It("sets EPINIO_SESSION and EPINIO_CSRF cookies", func() {
		c, w := newTestContext(http.MethodPost, "/api/v1/auth/login")

		Expect(session.SetSessionCookies(c, "admin")).To(Succeed())

		cookies := w.Result().Cookies()
		var sessionCookie, csrfCookie *http.Cookie
		for _, cookie := range cookies {
			switch cookie.Name {
			case session.SessionCookie:
				sessionCookie = cookie
			case session.CSRFCookie:
				csrfCookie = cookie
			}
		}

		Expect(sessionCookie).ToNot(BeNil())
		Expect(sessionCookie.HttpOnly).To(BeTrue())
		Expect(sessionCookie.Secure).To(BeTrue())
		Expect(sessionCookie.SameSite).To(Equal(http.SameSiteLaxMode))

		Expect(csrfCookie).ToNot(BeNil())
		Expect(csrfCookie.HttpOnly).To(BeFalse())
		Expect(csrfCookie.Value).ToNot(BeEmpty())

		claims, err := authtoken.ValidateSession(sessionCookie.Value)
		Expect(err).ToNot(HaveOccurred())
		Expect(claims.Username).To(Equal("admin"))
		Expect(claims.CSRF).To(Equal(csrfCookie.Value))
	})

	It("authenticates GET with session cookie and no CSRF header", func() {
		c, w := newTestContext(http.MethodGet, "/api/v1/me")
		Expect(session.SetSessionCookies(c, "admin")).To(Succeed())

		// Replay cookies onto a fresh GET request
		c2, _ := newTestContext(http.MethodGet, "/api/v1/me")
		for _, cookie := range w.Result().Cookies() {
			c2.Request.AddCookie(cookie)
		}

		username, err := session.AuthenticateCookie(c2)
		Expect(err).ToNot(HaveOccurred())
		Expect(username).To(Equal("admin"))
	})

	It("rejects POST without CSRF header", func() {
		c, w := newTestContext(http.MethodPost, "/api/v1/namespaces")
		Expect(session.SetSessionCookies(c, "admin")).To(Succeed())

		c2, _ := newTestContext(http.MethodPost, "/api/v1/namespaces")
		for _, cookie := range w.Result().Cookies() {
			c2.Request.AddCookie(cookie)
		}

		_, err := session.AuthenticateCookie(c2)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("csrf"))
	})

	It("accepts POST when X-Api-Csrf matches (including trailing =)", func() {
		c, w := newTestContext(http.MethodPost, "/api/v1/namespaces")
		Expect(session.SetSessionCookies(c, "admin")).To(Succeed())

		var csrf string
		for _, cookie := range w.Result().Cookies() {
			if cookie.Name == session.CSRFCookie {
				csrf = cookie.Value
			}
		}
		Expect(csrf).ToNot(BeEmpty())

		c2, _ := newTestContext(http.MethodPost, "/api/v1/namespaces")
		for _, cookie := range w.Result().Cookies() {
			c2.Request.AddCookie(cookie)
		}
		// UI historically appends "=" when forwarding the cookie.
		c2.Request.Header.Set("X-Api-Csrf", csrf+"=")

		username, err := session.AuthenticateCookie(c2)
		Expect(err).ToNot(HaveOccurred())
		Expect(username).To(Equal("admin"))
	})

	It("rejects POST with wrong CSRF header", func() {
		c, w := newTestContext(http.MethodPost, "/api/v1/namespaces")
		Expect(session.SetSessionCookies(c, "admin")).To(Succeed())

		c2, _ := newTestContext(http.MethodPost, "/api/v1/namespaces")
		for _, cookie := range w.Result().Cookies() {
			c2.Request.AddCookie(cookie)
		}
		c2.Request.Header.Set(session.CSRFHeader, "not-the-token")

		_, err := session.AuthenticateCookie(c2)
		Expect(err).To(HaveOccurred())
	})

	It("rejects missing session cookie", func() {
		c, _ := newTestContext(http.MethodGet, "/api/v1/me")
		_, err := session.AuthenticateCookie(c)
		Expect(err).To(HaveOccurred())
	})

	It("stores and reads OIDC flow cookie", func() {
		c, w := newTestContext(http.MethodGet, "/api/v1/auth/oidc/start")
		Expect(session.SetOIDCFlowCookie(c, "verifier-1", "state-1", "nonce-1")).To(Succeed())

		c2, _ := newTestContext(http.MethodGet, "/api/v1/auth/oidc/callback")
		for _, cookie := range w.Result().Cookies() {
			c2.Request.AddCookie(cookie)
		}

		verifier, state, nonce, err := session.ReadOIDCFlow(c2)
		Expect(err).ToNot(HaveOccurred())
		Expect(verifier).To(Equal("verifier-1"))
		Expect(state).To(Equal("state-1"))
		Expect(nonce).To(Equal("nonce-1"))
	})

	It("clears session, CSRF, and OIDC cookies", func() {
		c, w := newTestContext(http.MethodPost, "/api/v1/auth/logout")
		Expect(session.SetSessionCookies(c, "admin")).To(Succeed())
		Expect(session.SetOIDCFlowCookie(c, "v", "s", "n")).To(Succeed())

		session.ClearCookies(c)

		cleared := map[string]bool{}
		for _, cookie := range w.Result().Cookies() {
			if cookie.MaxAge < 0 || cookie.Value == "" {
				cleared[cookie.Name] = true
			}
		}
		Expect(cleared).To(HaveKey(session.SessionCookie))
		Expect(cleared).To(HaveKey(session.CSRFCookie))
		Expect(cleared).To(HaveKey(session.OIDCFlowCookie))
	})
})
