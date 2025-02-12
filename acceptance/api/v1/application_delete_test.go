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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppDelete Endpoint", LApplication, func() {
	var (
		namespace string
	)
	containerImageURL := "epinio/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	It("removes the application, unbinds bound configurations", func() {
		app1 := catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)
		configuration := catalog.NewConfigurationName()
		env.MakeConfiguration(configuration)
		env.BindAppConfiguration(app1, configuration, namespace)
		defer env.CleanupConfiguration(configuration)

		response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/namespaces/%s/applications/%s",
			serverURL, v1.Root, namespace, app1), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		Expect(response.StatusCode).To(Equal(http.StatusOK))
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())

		var resp map[string][]string
		err = json.Unmarshal(bodyBytes, &resp)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp).To(HaveLen(1))
		Expect(resp).To(HaveKey("unboundconfigurations"))
		Expect(resp["unboundconfigurations"]).To(ContainElement(configuration))
	})

	It("returns a 404 when the namespace does not exist", func() {
		app1 := catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)
		defer env.DeleteApp(app1)

		response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/namespaces/idontexist/applications/%s",
			serverURL, v1.Root, app1), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})

	It("returns a 404 when the app does not exist", func() {
		response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/namespaces/%s/applications/bogus",
			serverURL, v1.Root, namespace), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})
})
