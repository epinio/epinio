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

package machine

import (
	"os"
	"path"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (m *Machine) MakeGitconfig(gitConfigName string) {
	GinkgoHelper()

	m.MakeGitconfigWithoutCleanup(gitConfigName)

	DeferCleanup(func() {
		By("git config cleanup")
		out, err := proc.Kubectl(
			"delete", "secret", gitConfigName,
			"--namespace", "epinio",
		)
		Expect(err).NotTo(HaveOccurred(), out)
	})
}

func (m *Machine) MakeGitconfigWithoutCleanup(gitConfigName string) {
	GinkgoHelper()

	By("make git config")

	By("token directory and file")
	tmpTokenDir, err := os.MkdirTemp("", "tmp-token-dir")
	Expect(err).ToNot(HaveOccurred())

	tmpTokenFile := path.Join(tmpTokenDir, "token")
	err = os.WriteFile(tmpTokenFile, []byte(os.Getenv("PRIVATE_REPO_IMPORT_PAT")), 0600)
	Expect(err).ToNot(HaveOccurred())

	DeferCleanup(func() {
		By("token cleanup")
		os.RemoveAll(tmpTokenDir)
	})

	// create the secret with the github configuration
	By("creating the secret")
	out, err := proc.Kubectl(
		"create", "secret", "generic", gitConfigName,
		"--namespace", "epinio",
		"--from-literal=url=https://github.com",
		"--from-literal=username=anything",
		"--from-file=password="+tmpTokenFile,
	)
	Expect(err).NotTo(HaveOccurred(), out)

	// label the secret
	By("labeling the secret")
	out, err = proc.Kubectl(
		"label", "secret", gitConfigName,
		"--namespace", "epinio",
		"epinio.io/api-git-credentials=true",
	)
	Expect(err).NotTo(HaveOccurred(), out)
}
