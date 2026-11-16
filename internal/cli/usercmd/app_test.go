// Copyright © 2021 - 2026 KRUMWARE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package usercmd_test

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/internal/cli/usercmd/usercmdfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client Apps unit tests", Label("wip"), func() {
	var fake *usercmdfakes.FakeAPIClient

	Describe("AppDelete", func() {
		var epinioClient *usercmd.EpinioClient

		BeforeEach(func() {
			fake = &usercmdfakes.FakeAPIClient{}

			var err error
			epinioClient, err = usercmd.New()
			Expect(err).ToNot(HaveOccurred())

			epinioClient.Settings = &settings.Settings{Namespace: "workspace"}
			epinioClient.API = fake
		})

		When("deleting all applications", func() {
			It("discovers apps with AppMatchAll so undeployed apps are included", func() {
				fake.AppMatchAllReturns(models.AppMatchResponse{Names: []string{"built-only", "deployed"}}, nil)
				fake.AppDeleteReturns(models.ApplicationDeleteResponse{}, nil)

				err := epinioClient.AppDelete(context.Background(), nil, true, false, false)
				Expect(err).ToNot(HaveOccurred())

				Expect(fake.AppMatchAllCallCount()).To(BeNumerically(">=", 1))
				namespace, prefix := fake.AppMatchAllArgsForCall(0)
				Expect(namespace).To(Equal("workspace"))
				Expect(prefix).To(Equal(""))

				// Must not fall back to AppMatch (deployed-only).
				Expect(fake.AppMatchCallCount()).To(Equal(0))

				Expect(fake.AppDeleteCallCount()).To(Equal(1))
				deleteNS, names, deleteImage, deletePVC := fake.AppDeleteArgsForCall(0)
				Expect(deleteNS).To(Equal("workspace"))
				Expect(names).To(Equal([]string{"built-only", "deployed"}))
				Expect(deleteImage).To(BeFalse())
				Expect(deletePVC).To(BeFalse())
			})

			It("does not call AppDelete when AppMatchAll finds no apps", func() {
				fake.AppMatchAllReturns(models.AppMatchResponse{Names: []string{}}, nil)

				err := epinioClient.AppDelete(context.Background(), nil, true, false, false)
				Expect(err).ToNot(HaveOccurred())

				Expect(fake.AppMatchAllCallCount()).To(Equal(1))
				Expect(fake.AppMatchCallCount()).To(Equal(0))
				Expect(fake.AppDeleteCallCount()).To(Equal(0))
			})
		})
	})

	Describe("AppRestage", func() {

		When("restaging an existing app", func() {

			BeforeEach(func() {
				fake = &usercmdfakes.FakeAPIClient{}

				fake.AppShowStub = func(namespace, appName string) (models.App, error) {
					return *models.NewApp(appName, namespace), nil
				}

				fake.AppStageStub = func(
					req models.StageRequest,
				) (*models.AppOperation, error) {
					return &models.AppOperation{ID: "op1", StageID: "ID"}, nil
				}

				fake.AppLogsStub = func(namespace, appName, stageID string, follow bool, options *client.LogOptions, callback func(tailer.ContainerLogLine)) error {
					return nil
				}

				fake.AppStateStub = func(
					namespace string,
					appName string,
				) (models.AppState, error) {
					return models.AppState{Operation: &models.AppOperation{
						ID:    "op1",
						Phase: models.OperationSucceeded,
					}}, nil
				}
			})

			It("returns no error", func() {
				epinioClient, err := usercmd.New()
				Expect(err).ToNot(HaveOccurred())

				epinioClient.Settings = &settings.Settings{Namespace: "workspace"}
				epinioClient.API = fake

				err = epinioClient.AppRestage("appname", false)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("restaging a container-based app", func() {

			BeforeEach(func() {
				fake = &usercmdfakes.FakeAPIClient{}

				fake.AppShowStub = func(namespace, appName string) (models.App, error) {
					newApp := models.NewApp(appName, namespace)
					newApp.Origin = models.ApplicationOrigin{Kind: models.OriginContainer}
					return *newApp, nil
				}

				fake.AppStageStub = func(
					req models.StageRequest,
				) (*models.AppOperation, error) {
					panic("called AppStage!")
				}
			})

			It("returns no error and it won't stage", func() {
				epinioClient, err := usercmd.New()
				Expect(err).ToNot(HaveOccurred())

				epinioClient.Settings = &settings.Settings{Namespace: "workspace"}
				epinioClient.API = fake

				err = epinioClient.AppRestage("appname", false)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("restaging", func() {
			var running bool

			BeforeEach(func() {
				running = true
				fake = &usercmdfakes.FakeAPIClient{}

				fake.AppShowStub = func(
					namespace string,
					appName string,
				) (models.App, error) {
					app := models.NewApp(appName, namespace)
					if running {
						app.Workload = &models.AppDeployment{Active: true}
					}
					return *app, nil
				}
				op := &models.AppOperation{ID: "op1", StageID: "ID"}
				fake.AppDeployStub = func(
					models.DeployRequest,
				) (*models.AppOperation, error) {
					return op, nil
				}
				fake.AppStageStub = func(
					models.StageRequest,
				) (*models.AppOperation, error) {
					return op, nil
				}
				fake.AppLogsStub = func(
					namespace, appName, stageID string,
					follow bool,
					options *client.LogOptions,
					callback func(tailer.ContainerLogLine),
				) error {
					return nil
				}
				fake.AppStateStub = func(
					namespace string,
					appName string,
				) (models.AppState, error) {
					return models.AppState{Operation: &models.AppOperation{
						ID:    "op1",
						Phase: models.OperationSucceeded,
					}}, nil
				}
			})

			restage := func(restart bool) {
				epinioClient, err := usercmd.New()
				Expect(err).ToNot(HaveOccurred())

				epinioClient.Settings = &settings.Settings{
					Namespace: "workspace",
				}
				epinioClient.API = fake

				Expect(epinioClient.AppRestage("appname", restart)).
					To(Succeed())
			}

			It("builds and deploys a running app in one operation", func() {
				restage(true)

				Expect(fake.AppDeployCallCount()).To(Equal(1))
				Expect(fake.AppDeployArgsForCall(0).App.Name).
					To(Equal("appname"))
				Expect(fake.AppDeployArgsForCall(0).Builds()).To(BeTrue())
				Expect(fake.AppStageCallCount()).To(BeZero())
				Expect(fake.AppRestartCallCount()).To(BeZero())
			})

			It("only builds a running app with no-restart", func() {
				restage(false)

				Expect(fake.AppStageCallCount()).To(Equal(1))
				Expect(fake.AppDeployCallCount()).To(BeZero())
				Expect(fake.AppRestartCallCount()).To(BeZero())
			})

			It("only builds an app that is not running", func() {
				running = false
				restage(true)

				Expect(fake.AppStageCallCount()).To(Equal(1))
				Expect(fake.AppDeployCallCount()).To(BeZero())
				Expect(fake.AppRestartCallCount()).To(BeZero())
			})
		})
	})

	Describe("AppRestart waits for the operation", func() {
		var state models.AppState

		BeforeEach(func() {
			fake = &usercmdfakes.FakeAPIClient{}
			fake.AppRestartStub = func(
				namespace string,
				appName string,
			) (*models.AppOperation, error) {
				return &models.AppOperation{ID: "op1"}, nil
			}
			fake.AppStateStub = func(
				namespace string,
				appName string,
			) (models.AppState, error) {
				return state, nil
			}
		})

		restart := func() error {
			epinioClient, err := usercmd.New()
			Expect(err).ToNot(HaveOccurred())

			epinioClient.Settings = &settings.Settings{Namespace: "workspace"}
			epinioClient.API = fake

			return epinioClient.AppRestart("appname")
		}

		It("succeeds when the operation succeeds", func() {
			state = models.AppState{Operation: &models.AppOperation{
				ID:    "op1",
				Phase: models.OperationSucceeded,
			}}

			Expect(restart()).To(Succeed())
		})

		It("returns the server's error when the operation fails", func() {
			state = models.AppState{Operation: &models.AppOperation{
				ID:    "op1",
				Phase: models.OperationFailed,
				Error: "helm upgrade failed",
			}}

			Expect(restart()).To(MatchError("helm upgrade failed"))
		})

		It("fails when the server no longer tracks the operation", func() {
			state = models.AppState{}

			Expect(restart()).To(MatchError(
				ContainSubstring("no longer tracks the operation"),
			))
		})

		It("fails when another operation replaced it", func() {
			state = models.AppState{Operation: &models.AppOperation{
				ID:    "op2",
				Phase: models.OperationStaging,
			}}

			Expect(restart()).To(MatchError(
				ContainSubstring("another operation replaced this one"),
			))
		})
	})

	Describe("EnvSet waits for the redeploy it started", func() {
		var phase models.OperationPhase

		BeforeEach(func() {
			fake = &usercmdfakes.FakeAPIClient{}
			app := models.NewAppRef("appname", "workspace")
			fake.EnvSetReturns(
				models.OperationsResponse{
					Operations: []models.AppOperation{{ID: "op1", App: app}},
					Skipped: []models.SkippedApp{{
						App:    models.NewAppRef("busy", "workspace"),
						Reason: "busy",
					}},
				},
				nil,
			)
			fake.AppStateStub = func(
				namespace string,
				appName string,
			) (models.AppState, error) {
				return models.AppState{Operation: &models.AppOperation{
					ID:    "op1",
					Phase: phase,
					Error: "helm failed",
				}}, nil
			}
		})

		envSet := func() error {
			epinioClient, err := usercmd.New()
			Expect(err).ToNot(HaveOccurred())

			epinioClient.Settings = &settings.Settings{Namespace: "workspace"}
			epinioClient.API = fake

			return epinioClient.EnvSet(
				context.Background(),
				"appname",
				"KEY",
				"value",
			)
		}

		It("succeeds once the redeploy succeeds", func() {
			phase = models.OperationSucceeded

			Expect(envSet()).To(Succeed())
			Expect(fake.AppStateCallCount()).To(Equal(1))
		})

		It("names the app whose redeploy failed", func() {
			phase = models.OperationFailed

			Expect(envSet()).To(MatchError(
				"redeploy failed for appname: helm failed",
			))
		})
	})
})
