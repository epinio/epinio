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

package cmd_test

import (
	"github.com/epinio/epinio/internal/cli/cmd"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("--git-provider completion", func() {
	var completionFunc func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

	BeforeEach(func() {
		completionFunc = cmd.NewStaticFlagsCompletionFunc(models.ValidProviders)
	})

	It("returns all valid providers when toComplete is empty", func() {
		matches, directive := completionFunc(nil, nil, "")
		Expect(matches).To(ConsistOf("git", "github", "github_enterprise", "gitlab", "gitlab_enterprise"))
		Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
	})

	It("returns providers matching prefix 'git'", func() {
		matches, directive := completionFunc(nil, nil, "git")
		Expect(matches).To(ConsistOf("git", "github", "github_enterprise", "gitlab", "gitlab_enterprise"))
		Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
	})

	It("returns only github and github_enterprise for prefix 'github'", func() {
		matches, directive := completionFunc(nil, nil, "github")
		Expect(matches).To(ConsistOf("github", "github_enterprise"))
		Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
	})

	It("returns only github_enterprise for prefix 'github_en'", func() {
		matches, directive := completionFunc(nil, nil, "github_en")
		Expect(matches).To(Equal([]string{"github_enterprise"}))
		Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
	})

	It("returns only gitlab and gitlab_enterprise for prefix 'gitlab'", func() {
		matches, directive := completionFunc(nil, nil, "gitlab")
		Expect(matches).To(ConsistOf("gitlab", "gitlab_enterprise"))
		Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
	})

	It("returns no matches for unknown prefix 'bogus'", func() {
		matches, directive := completionFunc(nil, nil, "bogus")
		Expect(matches).To(BeEmpty())
		Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
	})

	It("returns single match for full provider name 'git'", func() {
		matches, directive := completionFunc(nil, nil, "git")
		Expect(matches).To(ContainElement("git"))
		Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
	})
})
