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
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppPart Endpoint", LApplication, func() {
	var (
		namespace string
		app       string
		domain    string
	)
	containerImageURL := "epinio/sample-app"

	// The testsuite checks using only part `values` and part `manifest`, as the smallest
	// possible parts, and also (YAML-formatted) text.  The data returned for parts `chart` and
	// `image` will be much much larger, and binary.

	BeforeEach(func() {
		domain = catalog.NewTmpName("exportdomain-") + ".org"

		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		app = catalog.NewAppName()
		env.MakeRoutedContainerImageApp(app, 1, containerImageURL, domain)
	})

	AfterEach(func() {
		env.DeleteApp(app)
		env.DeleteNamespace(namespace)
	})

	It("retrieves the named application part, values", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications/%s/part/values",
			serverURL, v1.Root, namespace, app), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()

		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		Expect(string(bodyBytes)).To(Equal(fmt.Sprintf(`epinio:
  appName: %s
  configpaths: []
  configurations: []
  env: []
  imageURL: epinio/sample-app
  ingress: null
  replicaCount: 1
  routes:
  - domain: %s
    id: %s
    path: /
  stageID: ""
  start: null
  tlsIssuer: epinio-ca
  username: admin
`, app, domain, domain)))
	})

	It("retrieves the named application part, manifest", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications/%s/part/manifest",
			serverURL, v1.Root, namespace, app), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()

		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		expecting := fmt.Sprintf(`name: %s
configuration:
    instances: 1
    routes:
        - %s
    appchart: standard
origin:
    container: epinio/sample-app
namespace: %s
`, app, domain, namespace)

		By(string(bodyBytes))
		By(expecting)
		Expect(string(bodyBytes)).To(Equal(expecting), string(bodyBytes))
	})

	It("returns a 404 when the namespace does not exist", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/idontexist/applications/%s/part/values",
			serverURL, v1.Root, app), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})

	It("returns a 404 when the app does not exist", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications/bogus/part/values",
			serverURL, v1.Root, namespace), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})

	It("returns a 400 when the part does not exist", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications/%s/part/bogus",
			serverURL, v1.Root, namespace, app), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
	})
})
