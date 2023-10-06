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

// Package auth collects structures and functions around the
// generation and processing of credentials.
package auth_test

import (
	"github.com/epinio/epinio/internal/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auth users", func() {

	Describe("IsAllowed", func() {

		var (
			appGetRole             auth.Role
			appGetNamespaceRole    auth.Role
			appDeleteRole          auth.Role
			appDeleteNamespaceRole auth.Role
		)

		BeforeEach(func() {
			appGetAction := auth.Action{
				Name: "app_get",
				Endpoints: []auth.Endpoint{
					{Method: "GET", BasePath: "/api/v1", Path: "/app"},
				},
			}

			appDeleteAction := auth.Action{
				Name: "app_delete",
				Endpoints: []auth.Endpoint{
					{Method: "DELETE", BasePath: "/api/v1", Path: "/app"},
				},
			}

			appGetRole = auth.Role{
				ID:      "app-get-role",
				Actions: []auth.Action{appGetAction},
			}
			appGetNamespaceRole = auth.Role{
				ID:        "app-get-role-workspace",
				Namespace: "workspace",
				Actions:   []auth.Action{appGetAction},
			}
			appDeleteRole = auth.Role{
				ID:      "app-delete-role",
				Actions: []auth.Action{appGetAction, appDeleteAction},
			}
			appDeleteNamespaceRole = auth.Role{
				ID:        "app-delete-role-workspace",
				Namespace: "workspace",
				Actions:   []auth.Action{appGetAction, appDeleteAction},
			}
		})

		When("the user has no roles", func() {

			It("is not allowed", func() {
				user := auth.User{}

				allowed := user.IsAllowed("GET", "/api/v1/app", nil)
				Expect(allowed).Should(BeFalse())

				params := map[string]string{"namespace": "workspace"}
				allowed = user.IsAllowed("DELETE", "/api/v1/app", params)
				Expect(allowed).Should(BeFalse())
			})
		})

		When("the user has a global admin role", func() {

			It("is allowed for every API requests", func() {
				user := auth.User{
					Roles: auth.Roles{{ID: "admin"}},
				}

				allowed := user.IsAllowed("GET", "/api/v1/app", nil)
				Expect(allowed).Should(BeTrue())

				params := map[string]string{"namespace": "workspace"}
				allowed = user.IsAllowed("DELETE", "/api/v1/app", params)
				Expect(allowed).Should(BeTrue())
			})
		})

		When("the user has admin role for a namespace", func() {

			It("is allowed only for namespaced requests", func() {
				user := auth.User{
					Roles: auth.Roles{{ID: "admin", Namespace: "workspace"}},
				}

				allowed := user.IsAllowed("GET", "/api/v1/app", nil)
				Expect(allowed).Should(BeFalse())

				params := map[string]string{"namespace": "workspace"}
				allowed = user.IsAllowed("DELETE", "/api/v1/app", params)
				Expect(allowed).Should(BeTrue())
			})
		})

		When("the user has all the roles", func() {

			var user auth.User

			BeforeEach(func() {
				user = auth.User{
					Roles: auth.Roles{
						appGetRole, appGetNamespaceRole,
						appDeleteRole, appDeleteNamespaceRole,
					},
				}
			})

			It("is allowed for every API requests", func() {
				allowed := user.IsAllowed("GET", "/api/v1/app", nil)
				Expect(allowed).Should(BeTrue())

				allowed = user.IsAllowed("DELETE", "/api/v1/app", nil)
				Expect(allowed).Should(BeTrue())
			})

			It("is allowed for every namespaced API requests", func() {
				params := map[string]string{"namespace": "workspace"}

				allowed := user.IsAllowed("GET", "/api/v1/app", params)
				Expect(allowed).Should(BeTrue())

				allowed = user.IsAllowed("DELETE", "/api/v1/app", params)
				Expect(allowed).Should(BeTrue())
			})

			It("is not allowed for unkwown API requests or methods", func() {
				allowed := user.IsAllowed("POST", "/api/v1/app", nil)
				Expect(allowed).Should(BeFalse())

				allowed = user.IsAllowed("POST", "/api/v1/unknown", nil)
				Expect(allowed).Should(BeFalse())
			})
		})

		When("the user has a global GET role but a namespaced DELETE role", func() {

			var user auth.User

			BeforeEach(func() {
				user = auth.User{
					Roles: auth.Roles{
						appGetRole, appDeleteNamespaceRole,
					},
				}
			})

			It("succeed to GET on a non namespaced endpoint", func() {
				allowed := user.IsAllowed("GET", "/api/v1/app", nil)
				Expect(allowed).Should(BeTrue())
			})

			It("fails to GET on a namespaced endpoint", func() {
				params := map[string]string{"namespace": "workspace"}
				allowed := user.IsAllowed("DELETE", "/api/v1/unknown", params)
				Expect(allowed).Should(BeFalse())
			})

			It("succeed to DELETE on a namespaced endpoint", func() {
				params := map[string]string{"namespace": "workspace"}
				allowed := user.IsAllowed("GET", "/api/v1/app", params)
				Expect(allowed).Should(BeTrue())
			})

			It("fails to DELETE on a non namespaced endpoint", func() {
				allowed := user.IsAllowed("DELETE", "/api/v1/unknown", nil)
				Expect(allowed).Should(BeFalse())
			})
		})
	})
})
