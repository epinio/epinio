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
				epinioClient, err := usercmd.NewEpinioClient(&settings.Settings{Namespace: "workspace"}, fake)
				Expect(err).ToNot(HaveOccurred())

				err = epinioClient.AppRestage("appname")
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
				epinioClient, err := usercmd.NewEpinioClient(&settings.Settings{Namespace: "workspace"}, fake)
				Expect(err).ToNot(HaveOccurred())

				err = epinioClient.AppRestage("appname")
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
