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

package usercmd_test

import (
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/internal/cli/usercmd/usercmdfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client Apps unit tests", Label("wip"), func() {
	var fake *usercmdfakes.FakeAPIClient

	Describe("AppRestage", func() {

		When("restaging an existing app", func() {

			BeforeEach(func() {
				fake = &usercmdfakes.FakeAPIClient{}

				fake.AppShowStub = func(namespace, appName string) (models.App, error) {
					return *models.NewApp(appName, namespace), nil
				}

				fake.AppStageStub = func(req models.StageRequest) (*models.StageResponse, error) {
					return &models.StageResponse{Stage: models.NewStage("ID")}, nil
				}

				fake.AppLogsStub = func(namespace, appName, stageID string, follow bool, callback func(tailer.ContainerLogLine)) error {
					return nil
				}

				fake.StagingCompleteStub = func(namespace, id string) (models.Response, error) {
					return models.Response{Status: "ok"}, nil
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

				fake.AppStageStub = func(req models.StageRequest) (*models.StageResponse, error) {
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
	})
})
