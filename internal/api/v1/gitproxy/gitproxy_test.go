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

package gitproxy_test

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/epinio/epinio/internal/api/v1/gitproxy"
	gitbridge "github.com/epinio/epinio/internal/bridge/git"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gitproxy Endpoint", func() {
	var w *httptest.ResponseRecorder
	var c *gin.Context
	var ctx context.Context

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)

		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		ctx = requestctx.WithLogger(context.Background(), logr.Discard())
	})

	When("JSON is malformed", func() {
		It("returns status code 400", func() {
			body := strings.NewReader(`{"url":"https://api.gi`)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "url", body)
			Expect(err).ToNot(HaveOccurred())
			c.Request = req

			gitManager := &gitbridge.Manager{Configurations: []gitbridge.Configuration{}}
			errs := gitproxy.Proxy(c, gitManager)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			Expect(errs).To(HaveOccurred())
		})
	})

	When("URL is malformed", func() {
		It("returns status code 400", func() {
			body := strings.NewReader(`{"url":"postgres://user:abc{DEf1=ghi@example.com:5432/db?sslmode=require"}`)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "url", body)
			Expect(err).ToNot(HaveOccurred())
			c.Request = req

			gitManager := &gitbridge.Manager{Configurations: []gitbridge.Configuration{}}
			errs := gitproxy.Proxy(c, gitManager)
			Expect(errs).To(HaveOccurred())
		})
	})

	When("proxying the request", func() {
		It("returns the same response", func() {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(201)
				fmt.Fprintf(w, `{"foo":"bar"}`)
			}))
			defer srv.Close()

			body := strings.NewReader(fmt.Sprintf(`{"url":"%s/repos/epinio/epinio","gitconfig":"missing"}`, srv.URL))
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "url", body)
			Expect(err).ToNot(HaveOccurred())
			c.Request = req

			gitManager := &gitbridge.Manager{Configurations: []gitbridge.Configuration{}}
			errs := gitproxy.Proxy(c, gitManager)
			Expect(errs).ToNot(HaveOccurred())
			Expect(w.Code).To(Equal(http.StatusCreated))

			b, err := io.ReadAll(w.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(b)).To(Equal(`{"foo":"bar"}`))
		})

		When("gitconfig with username and password is provided", func() {
			It("passes the BasicAuth header and returns the proxied response", func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					authHeader := r.Header.Get("Authorization")
					Expect(authHeader).To(Equal("Basic ZXBpbmlvOnBhc3N3b3Jk"))

					w.WriteHeader(201)
					fmt.Fprintf(w, `{"foo":"bar"}`)
				}))
				defer srv.Close()

				body := strings.NewReader(fmt.Sprintf(`{"url":"%s/repos/epinio/epinio","gitconfig":"my-git-conf"}`, srv.URL))
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "url", body)
				Expect(err).ToNot(HaveOccurred())
				c.Request = req

				gitManager := &gitbridge.Manager{Configurations: []gitbridge.Configuration{
					{ID: "my-git-conf", Username: "epinio", Password: "password"},
				}}
				errs := gitproxy.Proxy(c, gitManager)
				Expect(errs).ToNot(HaveOccurred())
				Expect(w.Code).To(Equal(http.StatusCreated))

				b, err := io.ReadAll(w.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(b)).To(Equal(`{"foo":"bar"}`))
			})
		})
	})

	When("proxying the request to HTTPS server", func() {
		When("no gitconfig is provided", func() {
			It("fails for unknown authority", func() {
				srv := httptest.NewTLSServer(nil)
				defer srv.Close()

				body := strings.NewReader(fmt.Sprintf(`{"url":"%s/repos/epinio/epinio"}`, srv.URL))
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "url", body)
				Expect(err).ToNot(HaveOccurred())
				c.Request = req

				gitManager := &gitbridge.Manager{Configurations: []gitbridge.Configuration{}}
				errs := gitproxy.Proxy(c, gitManager)
				Expect(errs).To(HaveOccurred())
			})
		})

		When("gitconfig with skipSSL is provided", func() {
			It("returns the proxied response", func() {
				srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					fmt.Fprintf(w, `{"foo":"bar"}`)
				}))
				defer srv.Close()

				body := strings.NewReader(fmt.Sprintf(`{"url":"%s/repos/epinio/epinio","gitconfig":"my-git-conf"}`, srv.URL))
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "url", body)
				Expect(err).ToNot(HaveOccurred())
				c.Request = req

				gitManager := &gitbridge.Manager{Configurations: []gitbridge.Configuration{
					{ID: "my-git-conf", SkipSSL: true},
				}}
				errs := gitproxy.Proxy(c, gitManager)
				Expect(errs).ToNot(HaveOccurred())
				Expect(w.Code).To(Equal(http.StatusCreated))

				b, err := io.ReadAll(w.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(b)).To(Equal(`{"foo":"bar"}`))
			})
		})

		When("gitconfig with the right CA bundle is provided", func() {
			It("returns the proxied response", func() {
				srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					fmt.Fprintf(w, `{"foo":"bar"}`)
				}))
				defer srv.Close()

				body := strings.NewReader(fmt.Sprintf(`{"url":"%s/repos/epinio/epinio","gitconfig":"my-git-conf"}`, srv.URL))
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "url", body)
				Expect(err).ToNot(HaveOccurred())
				c.Request = req

				conn, err := tls.Dial("tcp", strings.TrimPrefix(srv.URL, "https://"), &tls.Config{InsecureSkipVerify: true})
				Expect(err).ToNot(HaveOccurred())
				defer conn.Close()
				pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: conn.ConnectionState().PeerCertificates[0].Raw})

				gitManager := &gitbridge.Manager{Configurations: []gitbridge.Configuration{
					{ID: "my-git-conf", Certificate: pemCert},
				}}
				errs := gitproxy.Proxy(c, gitManager)
				Expect(errs).ToNot(HaveOccurred())
				Expect(w.Code).To(Equal(http.StatusCreated))

				b, err := io.ReadAll(w.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(b)).To(Equal(`{"foo":"bar"}`))
			})
		})
	})
})
