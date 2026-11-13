// Copyright © 2021 - 2026 SUSE LLC
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
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps Undeployed Endpoints", LApplication, func() {
	var namespace string

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	patchBuildFields := func(appName, buildStatus, imageURL string) {
		patch := fmt.Sprintf(
			`{"spec":{"buildstatus":%q,"imageurl":%q}}`,
			buildStatus, imageURL,
		)
		out, err := proc.Kubectl(
			"patch", "apps", appName,
			"--namespace", namespace,
			"--type", "merge",
			"-p", patch,
		)
		Expect(err).ToNot(HaveOccurred(), out)
	}

	createApp := func(name string) {
		bodyBytes, statusCode := appCreate(namespace, toJSON(models.ApplicationCreateRequest{
			Name: name,
			Configuration: models.ApplicationUpdateRequest{
				AppChart: "standard",
			},
		}))
		Expect(statusCode).To(Equal(http.StatusCreated), string(bodyBytes))
	}

	getApps := func(path string) models.AppList {
		response, err := env.Curl("GET", serverURL+v1.Root+"/"+path, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var apps models.AppList
		Expect(json.Unmarshal(bodyBytes, &apps)).To(Succeed())
		return apps
	}

	getPagedApps := func(path string) struct {
		Items      []models.App `json:"items"`
		Page       int          `json:"page"`
		PageSize   int          `json:"pageSize"`
		TotalItems int          `json:"totalItems"`
		TotalPages int          `json:"totalPages"`
	} {
		response, err := env.Curl("GET", serverURL+v1.Root+"/"+path, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var paged struct {
			Items      []models.App `json:"items"`
			Page       int          `json:"page"`
			PageSize   int          `json:"pageSize"`
			TotalItems int          `json:"totalItems"`
			TotalPages int          `json:"totalPages"`
		}
		Expect(json.Unmarshal(bodyBytes, &paged)).To(Succeed())
		return paged
	}

	namesOf := func(apps models.AppList) []string {
		names := make([]string, 0, len(apps))
		for _, app := range apps {
			names = append(names, app.Meta.Name)
		}
		return names
	}

	It("lists never-deployed apps on AppsUndeployed and AllAppsUndeployed", func() {
		createdOnly := catalog.NewAppName()
		neverDeployed := catalog.NewAppName()
		rebuilt := catalog.NewAppName()
		createApp(createdOnly)
		createApp(neverDeployed)
		createApp(rebuilt)
		defer env.DeleteApp(createdOnly)
		defer env.DeleteApp(neverDeployed)
		defer env.DeleteApp(rebuilt)

		// createdOnly keeps buildstatus=create from AppCreate
		patchBuildFields(neverDeployed, models.AppBuildStatusBuild, "")
		patchBuildFields(rebuilt, models.AppBuildStatusBuild, "registry.example/apps/rebuilt:abc")

		nsUndeployed := getApps(v1.Routes.Path("AppsUndeployed", namespace))
		Expect(namesOf(nsUndeployed)).To(ContainElements(createdOnly, neverDeployed, rebuilt))

		allUndeployed := getApps(v1.Routes.Path("AllAppsUndeployed"))
		Expect(namesOf(allUndeployed)).To(ContainElements(createdOnly, neverDeployed, rebuilt))

		// Never-deployed create/build stay off Apps; a rebuild that still has a
		// deployed imageurl appears on both undeployed and Apps lists.
		regular := getApps(v1.Routes.Path("Apps", namespace))
		Expect(namesOf(regular)).NotTo(ContainElements(createdOnly, neverDeployed))
		Expect(namesOf(regular)).To(ContainElement(rebuilt))
	})

	It("supports search and pagination on undeployed lists", func() {
		prefix := catalog.NewAppName()
		matching := prefix + "-match"
		other := catalog.NewAppName()
		createApp(matching)
		createApp(other)
		defer env.DeleteApp(matching)
		defer env.DeleteApp(other)

		patchBuildFields(matching, models.AppBuildStatusBuild, "")
		patchBuildFields(other, models.AppBuildStatusBuild, "")

		paged := getPagedApps(fmt.Sprintf(
			"%s?search=%s&page=1&pageSize=1",
			v1.Routes.Path("AppsUndeployed", namespace),
			prefix,
		))
		Expect(paged.Page).To(Equal(1))
		Expect(paged.PageSize).To(Equal(1))
		Expect(paged.TotalItems).To(Equal(1))
		Expect(paged.Items).To(HaveLen(1))
		Expect(paged.Items[0].Meta.Name).To(Equal(matching))

		allPaged := getPagedApps(fmt.Sprintf(
			"%s?search=%s&page=1&pageSize=10",
			v1.Routes.Path("AllAppsUndeployed"),
			prefix,
		))
		Expect(allPaged.TotalItems).To(BeNumerically(">=", 1))
		found := false
		for _, app := range allPaged.Items {
			if app.Meta.Name == matching {
				found = true
			}
		}
		Expect(found).To(BeTrue())
	})

	It("includes never-deployed apps in appsmatches only when includeUndeployed is set", func() {
		name := catalog.NewAppName()
		createApp(name)
		defer env.DeleteApp(name)
		patchBuildFields(name, models.AppBuildStatusBuild, "")

		matchNames := func(query string) []string {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/appsmatches%s",
				serverURL, v1.Root, namespace, query), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			var resp models.AppMatchResponse
			Expect(json.Unmarshal(bodyBytes, &resp)).To(Succeed())
			return resp.Names
		}

		Expect(matchNames("")).NotTo(ContainElement(name))
		Expect(matchNames("?includeUndeployed=true")).To(ContainElement(name))
	})

	Describe("a rebuild", func() {
		var name string

		pathOrigin := models.ApplicationOrigin{
			Kind: models.OriginPath,
			Path: testenv.TestAssetPath("sample-app.tar"),
		}

		BeforeEach(func() {
			name = catalog.NewAppName()
			createApp(name)
			DeferCleanup(func() { env.DeleteApp(name) })
		})

		It("keeps a redeploying app off the undeployed list", func() {
			request := models.DeployRequest{
				App:    models.NewAppRef(name, namespace),
				Origin: pathOrigin,
			}
			deployApplication(namespace, name, request)

			op := startedOperation(appDeploy(namespace, name, toJSON(request)))
			undeployed := v1.Routes.Path("AppsUndeployed", namespace)

			// The window is the whole build, so sample it until the
			// operation finishes rather than checking once.
			deadline := time.Now().Add(15 * time.Minute)
			for {
				Expect(namesOf(getApps(undeployed))).
					ToNot(ContainElement(name))

				state := appState(namespace, name)
				if state.Operation != nil &&
					state.Operation.ID == op.ID &&
					state.Operation.Done() {
					Expect(state.Operation.Phase).
						To(Equal(models.OperationSucceeded),
							state.Operation.Error)
					break
				}
				Expect(time.Now()).To(BeTemporally("<", deadline))
				time.Sleep(2 * time.Second)
			}

			shown := appShow(namespace, name)
			Expect(shown.BuildStatus).To(Equal(models.AppBuildStatusDeployed))
			Expect(shown.BuiltImageURL).To(Equal(shown.ImageURL))
		})

		It("does not report a failed build-only stage as built", func() {
			// The sample app has no Dockerfile, so the build Job fails.
			bodyBytes, statusCode := curl(
				http.MethodPost,
				makeEndpoint(v1.Routes.Path("AppStage", namespace, name)),
				toJSON(models.StageRequest{
					App:            models.NewAppRef(name, namespace),
					BlobUID:        uploadApplication(name, namespace).BlobUID,
					BuildMode:      models.BuildModeDockerfile,
					DockerfilePath: "Dockerfile",
				}),
			)
			op := startedOperation(bodyBytes, statusCode)
			done := waitForOperation(namespace, name, op.ID)
			Expect(done.Phase).To(Equal(models.OperationFailed))

			shown := appShow(namespace, name)
			Expect(shown.BuildStatus).To(Equal(models.AppBuildStatusCreate))
			Expect(shown.BuiltImageURL).To(BeEmpty())
			Expect(shown.Status).
				ToNot(Equal(models.ApplicationStatus(models.ApplicationBuilt)))
		})

		It("marks a successful build-only stage as built", func() {
			stageApplication(name, namespace, models.StageRequest{
				App:     models.NewAppRef(name, namespace),
				BlobUID: uploadApplication(name, namespace).BlobUID,
			})

			shown := appShow(namespace, name)
			Expect(shown.BuildStatus).To(Equal(models.AppBuildStatusBuild))
			Expect(shown.BuiltImageURL).ToNot(BeEmpty())
			Expect(shown.Status).
				To(Equal(models.ApplicationStatus(models.ApplicationBuilt)))
			Expect(namesOf(getApps(
				v1.Routes.Path("AppsUndeployed", namespace),
			))).To(ContainElement(name))
		})
	})
})
