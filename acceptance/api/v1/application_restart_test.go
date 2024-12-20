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
	"github.com/epinio/epinio/acceptance/helpers/proc"
	v1 "github.com/epinio/epinio/internal/api/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppRestart Endpoint", LApplication, func() {
	var (
		namespace string
		app1      string
	)
	containerImageURL := "epinio/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		app1 = catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)
	})

	AfterEach(func() {
		env.DeleteApp(app1)
		env.DeleteNamespace(namespace)
	})

	It("restarts the app", func() {
		getPodNames := func(namespace, app string) ([]string, error) {
			podName, err := proc.Kubectl("get", "pods", "-n", namespace, "-l", fmt.Sprintf("app.kubernetes.io/name=%s", app), "-o", "jsonpath='{.items[*].metadata.name}'")
			return strings.Split(podName, " "), err
		}

		oldPodNames, err := getPodNames(namespace, app1)
		Expect(err).ToNot(HaveOccurred())

		response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces/%s/applications/%s/restart",
			serverURL, v1.Root, namespace, app1), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		Expect(response.StatusCode).To(Equal(http.StatusOK))

		// Wait until pod changes name (restart is finished)
		Eventually(func() []string {
			names, err := getPodNames(namespace, app1)
			Expect(err).ToNot(HaveOccurred())
			return names
		}, "1m").ShouldNot(Equal(oldPodNames))
	})

	It("returns a 404 when the namespace does not exist", func() {
		response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces/idontexist/applications/%s/restart",
			serverURL, v1.Root, app1), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})

	It("returns a 404 when the app does not exist", func() {
		response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces/%s/applications/bogus/restart",
			serverURL, v1.Root, namespace), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})
})
