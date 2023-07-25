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

package models_test

import (
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GitProvider", func() {
	It("correctly converts a right provider from a correct string", func() {
		provider, err := models.GitProviderFromString("github")
		Expect(err).ToNot(HaveOccurred())
		Expect(provider).To(Equal(models.ProviderGithub))

		provider, err = models.GitProviderFromString("gitlab")
		Expect(err).ToNot(HaveOccurred())
		Expect(provider).To(Equal(models.ProviderGitlab))

		provider, err = models.GitProviderFromString("github_enterprise")
		Expect(err).ToNot(HaveOccurred())
		Expect(provider).To(Equal(models.ProviderGithubEnterprise))

		provider, err = models.GitProviderFromString("gitlab_enterprise")
		Expect(err).ToNot(HaveOccurred())
		Expect(provider).To(Equal(models.ProviderGitlabEnterprise))

		provider, err = models.GitProviderFromString("git")
		Expect(err).ToNot(HaveOccurred())
		Expect(provider).To(Equal(models.ProviderGit))
	})

	It("fails for an unknown provider string", func() {
		provider, err := models.GitProviderFromString("bogus")
		Expect(err).To(HaveOccurred())
		Expect(provider).To(Equal(models.ProviderUnknown))
	})

	It("has the right number of valid providers", func() {
		// This test will fail when we update the length of the valid providers.
		// This will remind us to update the tests, or the code, if needed.
		Expect(len(models.ValidProviders)).To(Equal(5))
	})

	It("does not fail for the right git URL, or unknown", func() {
		err := models.ProviderGithub.ValidateURL("https://github.com/user/repo")
		Expect(err).ToNot(HaveOccurred())

		err = models.ProviderGithub.ValidateURL("https://myprivate.github.com")
		Expect(err).ToNot(HaveOccurred())

		err = models.ProviderGitlab.ValidateURL("https://gitlab.com/user/repo")
		Expect(err).ToNot(HaveOccurred())

		err = models.ProviderGitlab.ValidateURL("https://myprivate.gitlab.com")
		Expect(err).ToNot(HaveOccurred())
	})

	It("fails parsing an incorrect git URL", func() {
		err := models.ProviderGit.ValidateURL("h://user:abc{DEf1=ghi@e")
		Expect(err).To(HaveOccurred())
	})

	It("fails for a mismatched provider and git URL", func() {
		err := models.ProviderGit.ValidateURL("https://github.com")
		Expect(err).To(HaveOccurred())

		err = models.ProviderGitlab.ValidateURL("https://github.com")
		Expect(err).To(HaveOccurred())

		err = models.ProviderGit.ValidateURL("https://gitlab.com")
		Expect(err).To(HaveOccurred())

		err = models.ProviderGithub.ValidateURL("https://gitlab.com")
		Expect(err).To(HaveOccurred())
	})
})
