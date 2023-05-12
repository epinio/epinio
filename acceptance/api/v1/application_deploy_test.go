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
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppDeploy Endpoint", LApplication, func() {
	var (
		namespace     string
		appName       string
		deployRequest models.DeployRequest
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()

		By("creating application resource first")
		_, err := createApplication(appName, namespace, nil)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		env.DeleteApp(appName)
		env.DeleteNamespace(namespace)
	})

	Context("with staging", func() {
		BeforeEach(func() {
			deployRequest = models.DeployRequest{
				App: models.AppRef{
					Meta: models.Meta{
						Name:      appName,
						Namespace: namespace,
					},
				},
				Origin: models.ApplicationOrigin{
					Kind: models.OriginPath,
					Path: testenv.TestAssetPath("sample-app.tar"),
				},
			}
		})

		When("staging, no other job for the same blob exists", func() {
			It("cleans up old S3 objects", func() {
				By("uploading the code")
				uploadResponse := uploadApplication(appName, namespace)
				oldBlob := uploadResponse.BlobUID

				stageRequest := models.StageRequest{
					App: models.AppRef{
						Meta: models.Meta{
							Name:      appName,
							Namespace: namespace,
						},
					},
					BlobUID:      oldBlob,
					BuilderImage: "paketobuildpacks/builder:full",
				}
				By("staging the application")
				_ = stageApplication(appName, namespace, stageRequest)
				Eventually(listS3Blobs, "1m").Should(ContainElement(ContainSubstring(oldBlob)))

				By("uploading the code again")
				uploadResponse = uploadApplication(appName, namespace)
				newBlob := uploadResponse.BlobUID

				By("staging the application again")
				stageRequest = models.StageRequest{
					App: models.AppRef{
						Meta: models.Meta{
							Name:      appName,
							Namespace: namespace,
						},
					},
					BlobUID:      newBlob,
					BuilderImage: "paketobuildpacks/builder:full",
				}
				stageResponse := stageApplication(appName, namespace, stageRequest)

				By("waiting for the new blob to appear")
				Eventually(listS3Blobs, "2m").Should(ContainElement(ContainSubstring(newBlob)))

				By("deploying the application")
				deployRequest.ImageURL = stageResponse.ImageURL
				deployRequest.Stage = stageResponse.Stage
				deployApplication(appName, namespace, deployRequest)

				By("waiting for the old blob to be gone")
				Eventually(listS3Blobs, "2m").ShouldNot(ContainElement(ContainSubstring(oldBlob)))
			})
		})

		When("an older staging job for the same blob exists", func() {
			It("doesn't delete the S3 object", func() {
				By("uploading the code")
				uploadResponse := uploadApplication(appName, namespace)
				theOnlyBlob := uploadResponse.BlobUID

				stageRequest := models.StageRequest{
					App: models.AppRef{
						Meta: models.Meta{
							Name:      appName,
							Namespace: namespace,
						},
					},
					BlobUID:      theOnlyBlob,
					BuilderImage: "paketobuildpacks/builder:full",
				}

				By("staging the application")
				_ = stageApplication(appName, namespace, stageRequest)
				Eventually(listS3Blobs, "1m").Should(ContainElement(ContainSubstring(theOnlyBlob)))

				By("staging the application again")
				stageResponse := stageApplication(appName, namespace, stageRequest)

				// sanity check
				out, err := proc.Kubectl("get", "Jobs",
					"--namespace", testenv.Namespace,
					"-o", "jsonpath={.items[*].metadata.labels['epinio\\.io/blob-uid']}")
				Expect(err).NotTo(HaveOccurred(), out)
				blobUIDs := strings.Split(out, " ")
				count := 0
				for _, b := range blobUIDs {
					if b == theOnlyBlob {
						count += 1
					}
				}
				Expect(count).To(Equal(2))

				deployRequest.ImageURL = stageResponse.ImageURL
				deployRequest.Stage.ID = stageResponse.Stage.ID
				deployApplication(appName, namespace, deployRequest)

				Consistently(listS3Blobs, "2m").Should(ContainElement(ContainSubstring(theOnlyBlob)))
			})
		})
	})

	Context("with non-staging using custom container image", func() {
		BeforeEach(func() {
			deployRequest = models.DeployRequest{
				App: models.AppRef{
					Meta: models.Meta{
						Name:      appName,
						Namespace: namespace,
					},
				},
				ImageURL: "splatform/sample-app",
				Origin: models.ApplicationOrigin{
					Kind:      models.OriginContainer,
					Container: "splatform/sample-app",
				},
			}
		})

		When("deploying a new app", func() {
			It("returns a success", func() {
				deployResponse := deployApplication(appName, namespace, deployRequest)
				Expect(deployResponse.Routes[0]).To(MatchRegexp(appName + `\..*\.omg\.howdoi\.website`))

				Eventually(func() string {
					return appFromAPI(namespace, appName).Workload.Status
				}, "5m").Should(Equal("1/1"))

				// Check if autoserviceaccounttoken is true
				labels := fmt.Sprintf("app.kubernetes.io/name=%s", appName)
				out, err := proc.Kubectl("get", "pod",
					"--namespace", namespace,
					"-l", labels,
					"-o", "jsonpath={.items[*].spec.automountServiceAccountToken}")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("true"))
			})
		})

		When("deploying an app with custom routes", func() {
			var routes []string

			BeforeEach(func() {
				routes = []string{"appdomain.org", "appdomain2.org"}
				out, err := proc.Kubectl("patch", "apps", "--type", "json",
					"-n", namespace, appName, "--patch",
					fmt.Sprintf(`[{"op": "replace", "path": "/spec/routes", "value": [%q, %q]}]`, routes[0], routes[1]))
				Expect(err).NotTo(HaveOccurred(), out)
			})

			It("the app Ingress matches the specified route", func() {
				// call the deploy action. Deploy should respect the routes on the App CR.
				deployApplication(appName, namespace, deployRequest)

				out, err := proc.Kubectl("get", "ingress",
					"--namespace", namespace, "-o", "jsonpath={.items[*].spec.rules[0].host}")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(strings.Split(out, " ")).To(ConsistOf(routes))
			})
		})

		When("deploying two apps with the same custom routes", func() {
			var routes []string
			var appName2 string

			BeforeEach(func() {
				appName2 = catalog.NewAppName()

				By("creating application resource first")
				_, err := createApplication(appName2, namespace, []string{})
				Expect(err).ToNot(HaveOccurred())

				routes = []string{"appdomain.org", "appdomain2.org"}
				out, err := proc.Kubectl("patch", "apps", "--type", "json", "-n", namespace, appName, "--patch",
					fmt.Sprintf(`[{"op": "replace", "path": "/spec/routes", "value": [%q, %q]}]`, routes[0], routes[1]))
				Expect(err).NotTo(HaveOccurred(), out)

				out, err = proc.Kubectl("patch", "apps", "--type", "json", "-n", namespace, appName2, "--patch",
					fmt.Sprintf(`[{"op": "replace", "path": "/spec/routes", "value": [%q, %q]}]`, routes[0], routes[1]))
				Expect(err).NotTo(HaveOccurred(), out)
			})

			AfterEach(func() {
				env.DeleteApp(appName2)
			})

			It("should fail the second deployment", func() {
				// call the deploy action. Deploy should respect the routes on the App CR.
				deployApplication(appName, namespace, deployRequest)
				deployRequest.App.Name = appName2
				deployApplicationWithFailure(appName2, namespace, deployRequest)

				out, err := proc.Kubectl("get", "ingress",
					"--namespace", namespace, "-o", "jsonpath={.items[*].spec.rules[0].host}")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(strings.Split(out, " ")).To(ConsistOf(routes))
			})
		})
	})

	Context("from git repository", func() {
		BeforeEach(func() {
			// Note: The deploy request is incomplete - no image url
			// That is ok, as it is used only to check a validation.
			// I.e. no actual deployment happens
			deployRequest = models.DeployRequest{
				App: models.AppRef{
					Meta: models.Meta{
						Name:      appName,
						Namespace: namespace,
					},
				},
				Origin: models.ApplicationOrigin{
					Kind: models.OriginGit,
					Git: &models.GitRef{
						URL: "https://github.com/epinio/example-wordpress",
					},
				},
			}
		})

		It("rejects a bad provider specification", func() {
			deployRequest.Origin.Git.Provider = "bogus"
			response := deployApplicationWithFailure(appName, namespace, deployRequest)
			Expect(response.Errors[0].Error()).To(ContainSubstring("bad git provider `bogus`"))
		})
	})
})
