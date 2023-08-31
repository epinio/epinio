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

package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gitconfigs", LGitconfig, func() {
	var gitconfigName string

	BeforeEach(func() {
		gitconfigName = catalog.NewGitconfigName()
	})

	Describe("gitconfig create", func() {
		AfterEach(func() {
			out, err := env.Epinio("", "gitconfig", "delete", gitconfigName)
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("creates a gitconfig", func() {
			out, err := env.Epinio("", "gitconfig", "create",
				gitconfigName, "url",
				"--username", "selfie",
				"--password", "pass",
				"--git-provider", "gitlab",
				"--skip-ssl",
				"--user-org", "anorg",
				"--repository", "therepo",
			)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Name: " + gitconfigName))
			Expect(out).To(ContainSubstring("Git configuration created"))

			out, err = env.Epinio("", "gitconfig", "show", gitconfigName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Name", gitconfigName),
					WithRow("Provider", "gitlab"),
					WithRow("URL", "url"),
					WithRow("User/Org", "anorg"),
					WithRow("Repository", "therepo"),
					WithRow("Skip SSL", "true"),
					WithRow("Username", "selfie"),
				),
			)
		})

		It("rejects creating an existing gitconfig", func() {
			out, err := env.Epinio("", "gitconfig", "create", gitconfigName, "url",
				"--username", "user",
				"--password", "pass")
			Expect(err).ToNot(HaveOccurred(), out)

			out, err = env.Epinio("", "gitconfig", "create", gitconfigName, "url",
				"--username", "user",
				"--password", "pass")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("gitconfig '%s' already exists", gitconfigName))
		})
	})

	Describe("gitconfig create failures", func() {
		It("rejects names not fitting kubernetes requirements", func() {
			gitconfigName := "BOGUS"
			out, err := env.Epinio("", "gitconfig", "create", gitconfigName, "url",
				"--username", "user",
				"--password", "pass")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("id must consist of lower case alphanumeric"))
		})

		It("rejects unknown git providers", func() {
			out, err := env.Epinio("", "gitconfig", "create",
				gitconfigName, "url",
				"--username", "user",
				"--password", "pass",
				"--git-provider", "bogus")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("unknown provider"))
		})
	})

	Describe("gitconfig list", func() {
		BeforeEach(func() {
			gitconfigName = catalog.NewGitconfigName()
			env.MakeGitconfig(gitconfigName)
			// Note: This also sets up the cleanup (dynamic after node)
		})

		It("lists gitconfigs", func() {
			out, err := env.Epinio("", "gitconfig", "list")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(
				HaveATable(
					WithHeaders("ID", "PROVIDER", "URL", "USER/ORG", "REPOSITORY", "SKIP SSL", "USERNAME"),
					WithRow(gitconfigName, "unknown", "https://github.com", "", "", "false", "anything"),
				),
			)
		})
	})

	Describe("gitconfig show", func() {
		BeforeEach(func() {
			gitconfigName = catalog.NewGitconfigName()
			env.MakeGitconfig(gitconfigName)
			// Note: This also sets up the cleanup (dynamic after node)
		})

		It("rejects showing an unknown gitconfig", func() {
			out, err := env.Epinio("", "gitconfig", "show", "missing-gitconfig")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("gitconfig 'missing-gitconfig' does not exist"))
		})

		Context("command completion", func() {
			It("matches empty prefix", func() {
				out, err := env.Epinio("", "__complete", "gitconfig", "show", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(gitconfigName))
			})

			It("does not match unknown prefix", func() {
				out, err := env.Epinio("", "__complete", "gitconfig", "show", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "gitconfig", "show", gitconfigName, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(gitconfigName))
			})
		})

		Context("existing gitconfig", func() {
			It("shows a gitconfig", func() {
				out, err := env.Epinio("", "gitconfig", "show", gitconfigName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Name", gitconfigName),
						WithRow("Provider", "unknown"),
						WithRow("URL", "https://github.com"),
						WithRow("User/Org", ""),
						WithRow("Repository", ""),
						WithRow("Skip SSL", "false"),
						WithRow("Username", "anything"),
					),
				)
			})
		})
	})

	Describe("gitconfig delete", func() {
		BeforeEach(func() {
			gitconfigName = catalog.NewGitconfigName()
			env.MakeGitconfigWithoutCleanup(gitconfigName)
			// Note: This also sets up the cleanup (dynamic after node)
		})

		It("deletes a git configuration", func() {
			out, err := env.Epinio("", "gitconfig", "delete", gitconfigName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Git configurations: %s", gitconfigName))
			Expect(out).To(ContainSubstring("Git configurations deleted."))
		})

		Context("command completion", func() {
			It("matches empty prefix", func() {
				out, err := env.Epinio("", "__complete", "gitconfig", "delete", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(gitconfigName))
			})

			It("does not match unknown prefix", func() {
				out, err := env.Epinio("", "__complete", "gitconfig", "delete", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "gitconfig", "delete", gitconfigName, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(gitconfigName))
			})
		})
	})
})
