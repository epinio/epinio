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

package acceptance_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Builder Image BoundApps", Label("builderimage"), func() {
	var imageName string

	BeforeEach(func() {
		imageName = catalog.NewTmpName("builderimage-ba-")
	})

	AfterEach(func() {
		// Best-effort cleanup of the builder image created by each spec.
		_, _ = env.Curl("DELETE", builderImagesURL(imageName), nil)
	})

	It("reports BoundApps=false for a builder image no application uses", func() {
		// A unique, unreferenced image string is deterministically unused by any
		// app, regardless of what else is running in the cluster.
		createBody, marshalError := json.Marshal(models.BuilderImageCreateRequest{
			Name:  imageName,
			Image: "registry.example.com/" + imageName + ":latest",
		})
		Expect(marshalError).ToNot(HaveOccurred())

		createResp, createError := env.Curl("POST", builderImagesURL(""), bytes.NewReader(createBody))
		Expect(createError).ToNot(HaveOccurred())
		decodeBody(createResp.Body, nil)
		Expect(createResp.StatusCode).To(Equal(http.StatusCreated))

		showResp, showError := env.Curl("GET", builderImagesURL(imageName), nil)
		Expect(showError).ToNot(HaveOccurred())
		var shown models.BuilderImage
		decodeBody(showResp.Body, &shown)
		Expect(showResp.StatusCode).To(Equal(http.StatusOK))
		Expect(shown.BoundApps).To(BeFalse())
	})

	It("reports BoundApps=true once an application is staged with the image", func() {
		By("discovering the cluster default builder image from /info")
		infoResp, infoError := env.Curl(
			"GET",
			strings.TrimSuffix(serverURL, "/")+"/api/v1/info",
			nil,
		)
		Expect(infoError).ToNot(HaveOccurred())
		var info models.InfoResponse
		decodeBody(infoResp.Body, &info)
		if info.DefaultBuilderImage == "" {
			Skip("no default builder image advertised by /info")
		}

		// Create a builder image whose image string equals the default, so the
		// app below (pushed without --builder-image) stages with exactly this
		// image. This keys BoundApps deterministically without depending on the
		// seeded default builder image's exact tag.
		By("creating a builder image matching the default")
		createBody, _ := json.Marshal(models.BuilderImageCreateRequest{
			Name:  imageName,
			Image: info.DefaultBuilderImage,
		})
		createResp, createError := env.Curl("POST", builderImagesURL(""), bytes.NewReader(createBody))
		Expect(createError).ToNot(HaveOccurred())
		decodeBody(createResp.Body, nil)
		Expect(createResp.StatusCode).To(Equal(http.StatusCreated))

		By("pushing an app, which stages with the default builder image")
		namespace := catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName := catalog.NewAppName()
		DeferCleanup(func() {
			env.DeleteApp(appName)
			env.DeleteNamespace(namespace)
		})

		pushOut, pushError := env.EpinioPush("../assets/sample-app", appName, "--name", appName)
		Expect(pushError).ToNot(HaveOccurred(), pushOut)

		By("GET reports BoundApps=true for the matching builder image")
		Eventually(func() bool {
			showResp, showError := env.Curl("GET", builderImagesURL(imageName), nil)
			Expect(showError).ToNot(HaveOccurred())
			var shown models.BuilderImage
			decodeBody(showResp.Body, &shown)
			return shown.BoundApps
		}, "2m", "5s").Should(BeTrue())
	})
})
