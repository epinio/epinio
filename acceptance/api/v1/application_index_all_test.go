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
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AllApps Endpoints", LApplication, func() {
	var (
		namespace1, namespace2 string
		app1, app2             string
		user, password         string
		containerImageURL      string
	)

	BeforeEach(func() {
		containerImageURL = "epinio/sample-app"

		namespace1 = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace1)

		app1 = catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)

		namespace2 = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace2)

		app2 = catalog.NewAppName()
		env.MakeContainerImageApp(app2, 1, containerImageURL)

		user, password = env.CreateEpinioUser("user", nil)
	})

	AfterEach(func() {
		env.TargetNamespace(namespace2)
		env.DeleteApp(app2)

		env.TargetNamespace(namespace1)
		env.DeleteApp(app1)

		env.DeleteNamespace(namespace1)
		env.DeleteNamespace(namespace2)

		env.DeleteEpinioUser(user)
	})

	It("lists all applications belonging to all namespaces", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/applications",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var apps models.AppList
		err = json.Unmarshal(bodyBytes, &apps)
		Expect(err).ToNot(HaveOccurred())

		// `apps` contains all apps. Not just the two we are looking for, from
		// the setup of this test. Everything which still exists from other
		// tests executing concurrently, or not cleaned by previous tests, or
		// the setup, or ... So, we cannot be sure that the two apps are in the
		// two first elements of the slice.

		var appRefs [][]string
		for _, a := range apps {
			appRefs = append(appRefs, []string{a.Meta.Name, a.Meta.Namespace})
		}
		Expect(appRefs).To(ContainElements(
			[]string{app1, namespace1},
			[]string{app2, namespace2}))
	})

	It("doesn't list applications belonging to non-accessible namespaces", func() {
		endpoint := fmt.Sprintf("%s%s/applications", serverURL, v1.Root)
		request, err := http.NewRequest(http.MethodGet, endpoint, nil)
		Expect(err).ToNot(HaveOccurred())
		request.SetBasicAuth(user, password)

		response, err := env.Client().Do(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var apps models.AppList
		err = json.Unmarshal(bodyBytes, &apps)
		Expect(err).ToNot(HaveOccurred())
		Expect(apps).To(BeEmpty())
	})
})
