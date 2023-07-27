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
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppImportGit Endpoint", LApplication, func() {

	const gitURL = "https://github.com/epinio/example-wordpress"

	var (
		namespace string
		app       string
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		app = catalog.NewAppName()

		DeferCleanup(func() {
			env.DeleteNamespace(namespace)
		})
	})

	Describe("POST /namespaces/:namespace/applications/:app/import-git", func() {

		It("imports the git repo in the blob store without specifying revision", func() {
			revision := ""

			bodyBytes, statusCode := appImportGit(namespace, app, gitURL, revision)
			Expect(statusCode).To(Equal(http.StatusOK), string(bodyBytes))

			importResponse := fromJSON[models.ImportGitResponse](bodyBytes)
			Expect(importResponse.BlobUID).ToNot(BeEmpty())
			Expect(importResponse.BlobUID).To(BeUUID())
			Expect(importResponse.Branch).To(Equal("main"))
			Expect(importResponse.Revision).ToNot(BeEmpty())
		})

		It("imports the git repo in the blob store from a branch", func() {
			revision := "main"

			bodyBytes, statusCode := appImportGit(namespace, app, gitURL, revision)
			Expect(statusCode).To(Equal(http.StatusOK), string(bodyBytes))

			importResponse := fromJSON[models.ImportGitResponse](bodyBytes)
			Expect(importResponse.BlobUID).ToNot(BeEmpty())
			Expect(importResponse.BlobUID).To(BeUUID())
			Expect(importResponse.Branch).To(Equal("main"))
			Expect(importResponse.Revision).ToNot(BeEmpty())
		})

		It("imports the git repo in the blob store from a revision", func() {
			revision := "48c02bd5766061c0ea9875ca1fd9908e3a20aeb8"

			bodyBytes, statusCode := appImportGit(namespace, app, gitURL, revision)
			Expect(statusCode).To(Equal(http.StatusOK), string(bodyBytes))

			importResponse := fromJSON[models.ImportGitResponse](bodyBytes)
			Expect(importResponse.BlobUID).ToNot(BeEmpty())
			Expect(importResponse.BlobUID).To(BeUUID())
			Expect(importResponse.Branch).ToNot(BeEmpty())
			Expect(importResponse.Revision).To(Equal("48c02bd5766061c0ea9875ca1fd9908e3a20aeb8"))
		})

		It("imports the git repo in the blob store from a short commit revision", func() {
			revision := "48c02bd"

			bodyBytes, statusCode := appImportGit(namespace, app, gitURL, revision)
			Expect(statusCode).To(Equal(http.StatusOK), string(bodyBytes))

			importResponse := fromJSON[models.ImportGitResponse](bodyBytes)
			Expect(importResponse.BlobUID).ToNot(BeEmpty())
			Expect(importResponse.BlobUID).To(BeUUID())
			Expect(importResponse.Branch).ToNot(BeEmpty())
			Expect(importResponse.Revision).To(Equal("48c02bd5766061c0ea9875ca1fd9908e3a20aeb8"))
		})
	})
})
