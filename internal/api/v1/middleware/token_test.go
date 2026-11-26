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

package middleware_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/epinio/epinio/helpers/authtoken"
	"github.com/epinio/epinio/internal/api/v1/middleware"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TokenAuth", func() {
	runTokenAuth := func(token string) *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/ws?authtoken="+token, nil)
		middleware.TokenAuth(c)
		return w
	}

	It("rejects a session JWT used as websocket authtoken", func() {
		sessionJWT := authtoken.CreateSession("armin", "csrf-token", authtoken.GetSessionExpiry())
		Expect(sessionJWT).NotTo(BeEmpty())

		w := runTokenAuth(sessionJWT)
		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})

	It("rejects an OIDC-flow JWT used as websocket authtoken", func() {
		flowJWT := authtoken.CreateOIDCFlow("verifier", "state", "nonce", authtoken.MaxOIDCFlowExpiry)
		Expect(flowJWT).NotTo(BeEmpty())

		w := runTokenAuth(flowJWT)
		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})
})
