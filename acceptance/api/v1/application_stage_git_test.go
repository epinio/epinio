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

package v1_test

import (
	"net/http"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Staging from a git origin replaced POST .../import-git: the server clones
// the repository inside the build operation and stamps the resolved branch
// and commit onto the app's origin. Clone and stamp both happen before the
// staging Job exists, so these specs wait only for the stage ID, not for
// the build itself.
var _ = Describe("AppStage from a git origin", LApplication, func() {

	const gitURL = "https://github.com/epinio/example-wordpress"
	const exampleGoURL = "https://github.com/epinio/example-go"

	var (
		namespace string
		app       string
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		app = catalog.NewAppName()

		bodyBytes, statusCode := appCreate(namespace,
			toJSON(models.ApplicationCreateRequest{Name: app}))
		Expect(statusCode).To(Equal(http.StatusCreated), string(bodyBytes))

		DeferCleanup(func() {
			env.DeleteNamespace(namespace)
		})
	})

	stageGit := func(url, revision string) ([]byte, int) {
		request := models.StageRequest{
			App: models.NewAppRef(app, namespace),
			Origin: models.ApplicationOrigin{
				Kind: models.OriginGit,
				Git:  &models.GitRef{URL: url, Revision: revision},
			},
		}
		endpoint := makeEndpoint(v1.Routes.Path("AppStage", namespace, app))
		return curl(http.MethodPost, endpoint, toJSON(request))
	}

	// clonedOrigin stages from git, waits until the clone is done (the
	// operation reports its stage), and returns the app as stored.
	clonedOrigin := func(url, revision string) models.App {
		op := startedOperation(stageGit(url, revision))

		Eventually(func(g Gomega) {
			state := appState(namespace, app)
			g.Expect(state.Operation).ToNot(BeNil())
			g.Expect(state.Operation.ID).To(Equal(op.ID))
			g.Expect(state.Operation.Error).To(BeEmpty())
			g.Expect(state.Operation.StageID).ToNot(BeEmpty())
		}, "5m", "2s").Should(Succeed())

		stored := appShow(namespace, app)
		Expect(stored.Origin.Kind).To(Equal(models.OriginGit))
		Expect(stored.Origin.Git).ToNot(BeNil())
		Expect(stored.Origin.Git.URL).To(Equal(url))
		Expect(stored.BlobUID).To(BeUUID())

		return stored
	}

	It("rejects a missing git url", func() {
		bodyBytes, statusCode := stageGit("", "")
		ExpectBadRequestError(bodyBytes, statusCode, "missing giturl")
	})

	It("rejects a malformed git url", func() {
		bodyBytes, statusCode := stageGit("github.com", "")
		ExpectBadRequestError(bodyBytes, statusCode,
			"missing scheme or host in giturl [://]")

		bodyBytes, statusCode = stageGit("//github.com", "")
		ExpectBadRequestError(bodyBytes, statusCode,
			"missing scheme or host in giturl [://github.com]")

		bodyBytes, statusCode = stageGit("git://", "")
		ExpectBadRequestError(bodyBytes, statusCode,
			"missing scheme or host in giturl [git://]")
	})

	It("fails the operation for a revision that does not exist", func() {
		op := startedOperation(stageGit(gitURL, "non-existing"))

		done := waitForOperation(namespace, app, op.ID)
		Expect(done.Phase).To(Equal(models.OperationFailed))
		Expect(done.Error).To(ContainSubstring("reference not found"))
	})

	It("stamps the default branch and its commit without a revision", func() {
		git := clonedOrigin(gitURL, "").Origin.Git
		Expect(git.Branch).To(Equal("main"))
		Expect(git.Revision).To(HaveLen(40))
	})

	It("stamps the commit of a branch", func() {
		git := clonedOrigin(gitURL, "main").Origin.Git
		Expect(git.Branch).To(Equal("main"))
		Expect(git.Revision).To(HaveLen(40))
	})

	It("keeps a full commit revision", func() {
		revision := "48c02bd5766061c0ea9875ca1fd9908e3a20aeb8"

		git := clonedOrigin(gitURL, revision).Origin.Git
		Expect(git.Branch).ToNot(BeEmpty())
		Expect(git.Revision).To(Equal(revision))
	})

	It("expands a short commit revision", func() {
		git := clonedOrigin(gitURL, "48c02bd").Origin.Git
		Expect(git.Revision).To(
			Equal("48c02bd5766061c0ea9875ca1fd9908e3a20aeb8"),
		)
	})

	It("resolves a tag to its commit", func() {
		git := clonedOrigin(exampleGoURL, "v0.0.2").Origin.Git
		// Tag checkout is detached HEAD; go-git may report "HEAD" or "main"
		Expect(git.Branch).To(BeElementOf("main", "HEAD"))
		Expect(git.Revision).To(
			Equal("e84b2a73b2c1bb88d9cdc99ffca1a3d05b3d261b"),
		)
	})

	It("resolves a commit and its branch", func() {
		revision := "15e2b2690ac9b372963544384b9aa43955a2e611"

		git := clonedOrigin(exampleGoURL, revision).Origin.Git
		// Commit checkout may be detached HEAD; go-git may report "HEAD",
		// the branch name, or a tag
		Expect(git.Branch).To(BeElementOf("test", "HEAD", "v0.0.1"))
		Expect(git.Revision).To(Equal(revision))
	})
})
