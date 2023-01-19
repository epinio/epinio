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
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppImportGit Endpoint", LApplication, func() {
	var (
		namespace string
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	Describe("POST /namespaces/:namespace/applications/:app/import-git", func() {
		It("imports the git repo in the blob store", func() {
			app := catalog.NewAppName()
			gitURL := "https://github.com/epinio/example-wordpress"
			data := url.Values{}
			data.Set("giturl", gitURL)
			data.Set("gitrev", "main")

			url := serverURL + v1.Root + "/" + v1.Routes.Path("AppImportGit", namespace, app)
			request, err := http.NewRequest("POST", url, strings.NewReader(data.Encode()))
			Expect(err).ToNot(HaveOccurred())
			request.SetBasicAuth(env.EpinioUser, env.EpinioPassword)
			request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

			response, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred(), string(bodyBytes))
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			var importResponse models.ImportGitResponse
			err = json.Unmarshal(bodyBytes, &importResponse)
			Expect(err).ToNot(HaveOccurred())
			Expect(importResponse.BlobUID).ToNot(BeEmpty())
			Expect(importResponse.BlobUID).To(MatchRegexp(".+-.+-.+-.+-.+"))
		})
	})
})
