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

	setupRequestBody := func(body string) {
		GinkgoHelper()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "url", strings.NewReader(body))
		Expect(err).ToNot(HaveOccurred())
		c.Request = req
	}

	When("JSON is malformed", func() {
		It("returns status code 400", func() {
			setupRequestBody(`{"url":"https://api.gi`)

			gitManager := &gitbridge.Manager{Configurations: []gitbridge.Configuration{}}

			errs := gitproxy.Proxy(c, gitManager)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			Expect(errs).To(HaveOccurred())
		})
	})

	When("URL is malformed", func() {
		It("returns status code 400", func() {
			setupRequestBody(`{"url":"postgres://user:abc{DEf1=ghi@example.com:5432/db?sslmode=require"}`)

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

			setupRequestBody(fmt.Sprintf(`{"url":"%s/api/v3/repos/epinio/epinio","gitconfig":"missing"}`, srv.URL))

			gitManager := &gitbridge.Manager{Configurations: []gitbridge.Configuration{}}

			errs := gitproxy.Proxy(c, gitManager)
			Expect(errs).ToNot(HaveOccurred())
			Expect(w.Code).To(Equal(http.StatusCreated))

			b, err := io.ReadAll(w.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(b)).To(Equal(`{"foo":"bar"}`))
		})

		It("fails for unkwnown URLs", func() {
			setupAndRun := func(path string) {
				GinkgoHelper()

				err := gitproxy.ValidateURL(fmt.Sprintf("https://mydomain.com%s", path))
				Expect(err).To(HaveOccurred())
			}

			setupAndRun("")
			setupAndRun("/apis")
		})

		It("fails for Github non whitelisted URLs", func() {
			setupAndRun := func(path string) {
				GinkgoHelper()

				err := gitproxy.ValidateURL(fmt.Sprintf("https://api.github.com%s", path))
				Expect(err).To(HaveOccurred())
			}

			setupAndRun("")
			setupAndRun("/")
			setupAndRun("/epinio")
			setupAndRun("/epinio/epinio")
			setupAndRun("/epinio/with/parts")
			setupAndRun("/epinio/with/four/parts")
			setupAndRun("/epinio/with/five/parts/here")
			setupAndRun("/epinio/with/too/many/parts/here")
		})

		It("passes for Github whitelisted URLs", func() {
			// - /repos/USERNAME/REPO
			// - /repos/USERNAME/REPO/commits
			// - /repos/USERNAME/REPO/branches
			// - /repos/USERNAME/REPO/branches/BRANCH
			// - /users/USERNAME/repos
			// - /search/repositories

			setupAndRun := func(path string) {
				GinkgoHelper()

				err := gitproxy.ValidateURL(fmt.Sprintf("https://api.github.com%s", path))
				Expect(err).ToNot(HaveOccurred())
			}

			setupAndRun("/search/repositories?q=repo:USER/ORG")
			setupAndRun("/users/USERNAME/repos")
			setupAndRun("/repos/USERNAME/REPO")
			setupAndRun("/repos/USERNAME/REPO/commits")
			setupAndRun("/repos/USERNAME/REPO/branches")
			setupAndRun("/repos/USERNAME/REPO/branches/BRANCH")
		})

		It("passes for Github Enterprise Server whitelisted URLs", func() {
			// - /api/v3/repos/USERNAME/REPO
			// - /api/v3/repos/USERNAME/REPO/commits
			// - /api/v3/repos/USERNAME/REPO/branches
			// - /api/v3/repos/USERNAME/REPO/branches/BRANCH
			// - /api/v3/users/USERNAME/repos
			// - /api/v3/search/repositories

			setupAndRun := func(path string) {
				GinkgoHelper()

				err := gitproxy.ValidateURL(fmt.Sprintf("https://github.mydomain.com/api/v3%s", path))
				Expect(err).ToNot(HaveOccurred())
			}

			setupAndRun("/search/repositories")
			setupAndRun("/users/USERNAME/repos")
			setupAndRun("/repos/USERNAME/REPO")
			setupAndRun("/repos/USERNAME/REPO/commits")
			setupAndRun("/repos/USERNAME/REPO/branches")
			setupAndRun("/repos/USERNAME/REPO/branches/BRANCH")
		})

		It("fails for Gitlab Server non whitelisted URLs", func() {
			setupAndRun := func(path string) {
				GinkgoHelper()

				err := gitproxy.ValidateURL(fmt.Sprintf("https://gitlab.mydomain.com/api/v4%s", path))
				Expect(err).To(HaveOccurred())
			}

			setupAndRun("")
			setupAndRun("/")
			setupAndRun("/any")
			setupAndRun("/search/something")
			setupAndRun("/USERNAME/projects")
			setupAndRun("/projects/USERNAME%2FREPO/repository/stars")
			setupAndRun("/projects/USERNAME%2FREPO/repository/commits/BRANCH")
			setupAndRun("/api/with/too/many/parts/api")
		})

		It("passes for Gitlab Server whitelisted URLs", func() {
			// - /api/v4/avatar
			// - /api/v4/search/repositories
			// - /api/v4/users/USERNAME/projects
			// - /api/v4/groups/USERNAME/projects
			// - /api/v4/projects/USERNAME%2FREPO
			// - /api/v4/projects/USERNAME%2FREPO/repository/commits
			// - /api/v4/projects/USERNAME%2FREPO/repository/branches
			// - /api/v4/projects/USERNAME%2FREPO/repository/branches/BRANCH

			setupAndRun := func(path string) {
				GinkgoHelper()

				err := gitproxy.ValidateURL(fmt.Sprintf("https://gitlab.mydomain.com/api/v4%s", path))
				Expect(err).ToNot(HaveOccurred())
			}

			setupAndRun("/avatar")
			setupAndRun("/search/repositories")
			setupAndRun("/users/USERNAME/projects")
			setupAndRun("/groups/USERNAME/projects")
			setupAndRun("/projects/USERNAME%2FREPO")
			setupAndRun("/projects/USERNAME%2FREPO/repository/commits")
			setupAndRun("/projects/USERNAME%2FREPO/repository/branches")
			setupAndRun("/projects/USERNAME%2FREPO/repository/branches/BRANCH")
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

				gitConfig := "my-git-conf"
				setupRequestBody(fmt.Sprintf(`{"url":"%s/api/v3/repos/epinio/epinio","gitconfig":"%s"}`, srv.URL, gitConfig))

				gitManager := &gitbridge.Manager{Configurations: []gitbridge.Configuration{
					{ID: gitConfig, Username: "epinio", Password: "password"},
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

				setupRequestBody(fmt.Sprintf(`{"url":"%s/api/v3/repos/epinio/epinio"}`, srv.URL))

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

				gitConfig := "my-git-conf"
				setupRequestBody(fmt.Sprintf(`{"url":"%s/api/v3/repos/epinio/epinio","gitconfig":"%s"}`, srv.URL, gitConfig))

				gitManager := &gitbridge.Manager{Configurations: []gitbridge.Configuration{
					{ID: gitConfig, SkipSSL: true},
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

				gitConfig := "my-git-conf"
				setupRequestBody(fmt.Sprintf(`{"url":"%s/api/v3/repos/epinio/epinio","gitconfig":"%s"}`, srv.URL, gitConfig))

				// load PEM certificate from TLS server
				conn, err := tls.Dial("tcp", strings.TrimPrefix(srv.URL, "https://"), &tls.Config{InsecureSkipVerify: true})
				Expect(err).ToNot(HaveOccurred())
				defer conn.Close()
				pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: conn.ConnectionState().PeerCertificates[0].Raw})

				gitManager := &gitbridge.Manager{Configurations: []gitbridge.Configuration{
					{ID: gitConfig, Certificate: pemCert},
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
