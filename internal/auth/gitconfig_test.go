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

// fakeGitconfig is a minimal auth.GitconfigResource for filter/permission tests.
type fakeGitconfig struct {
	id     string
	global bool
}

func (f fakeGitconfig) Gitconfig() string { return f.id }
func (f fakeGitconfig) IsGlobal() bool    { return f.global }

var adminRole = auth.Role{ID: "admin"}

var _ = Describe("Gitconfig authorization", func() {
	owned := fakeGitconfig{id: "owned"}
	global := fakeGitconfig{id: "global", global: true}
	other := fakeGitconfig{id: "other"}
	all := []fakeGitconfig{owned, global, other}

	Describe("FilterGitconfigResources", func() {
		It("returns everything for an admin", func() {
			user := auth.User{Roles: auth.Roles{adminRole}}

			Expect(auth.FilterGitconfigResources(user, all)).To(Equal(all))
		})

		It("returns owned and global configs for a regular user", func() {
			user := auth.User{Gitconfigs: []string{"owned"}}

			Expect(auth.FilterGitconfigResources(user, all)).
				To(ConsistOf(owned, global))
		})

		It("returns only global configs when the user owns none", func() {
			user := auth.User{}

			Expect(auth.FilterGitconfigResources(user, all)).
				To(ConsistOf(global))
		})
	})

	Describe("CanDeleteGitconfig", func() {
		It("allows an admin to delete any config", func() {
			user := auth.User{Roles: auth.Roles{adminRole}}

			Expect(auth.CanDeleteGitconfig(user, other)).To(BeTrue())
			Expect(auth.CanDeleteGitconfig(user, global)).To(BeTrue())
		})

		It("allows a user to delete a config they own", func() {
			user := auth.User{Gitconfigs: []string{"owned"}}

			Expect(auth.CanDeleteGitconfig(user, owned)).To(BeTrue())
		})

		It("does not let a non-owner delete a global config", func() {
			user := auth.User{Gitconfigs: []string{"owned"}}

			Expect(auth.CanDeleteGitconfig(user, global)).To(BeFalse())
		})

		It("does not let a user delete a config they do not own", func() {
			user := auth.User{Gitconfigs: []string{"owned"}}

			Expect(auth.CanDeleteGitconfig(user, other)).To(BeFalse())
		})
	})
})
