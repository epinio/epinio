package usercmd_test

import (
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/cli/usercmd"
	epinioapi "github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kubectlterm "k8s.io/kubectl/pkg/util/term"
)

var _ = Describe("Client Apps unit tests", Label("wip"), func() {

	Describe("AppRestage", func() {

		When("restaging an existing app", func() {
			var mockClient *mockAPIClient

			BeforeEach(func() {
				mockClient = &mockAPIClient{}

				mockClient.mockAppShow = func(namespace, appName string) (models.App, error) {
					return *models.NewApp(appName, namespace), nil
				}

				mockClient.mockAppStage = func(req models.StageRequest) (*models.StageResponse, error) {
					return &models.StageResponse{Stage: models.NewStage("ID")}, nil
				}

				mockClient.mockAppLogs = func(namespace, appName, stageID string, follow bool, callback func(tailer.ContainerLogLine)) error {
					return nil
				}

				mockClient.mockStagingComplete = func(namespace, id string) (models.Response, error) {
					return models.Response{Status: "ok"}, nil
				}
			})

			It("returns no error", func() {
				epinioClient, err := usercmd.NewEpinioClient(&settings.Settings{Namespace: "workspace"}, mockClient)
				Expect(err).ToNot(HaveOccurred())

				err = epinioClient.AppRestage("appname")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("restaging a container-based app", func() {
			var mockClient *mockAPIClient

			BeforeEach(func() {
				mockClient = &mockAPIClient{}

				mockClient.mockAppShow = func(namespace, appName string) (models.App, error) {
					newApp := models.NewApp(appName, namespace)
					newApp.Origin = models.ApplicationOrigin{Kind: models.OriginContainer}
					return *newApp, nil
				}

				mockClient.mockAppStage = func(req models.StageRequest) (*models.StageResponse, error) {
					panic("called AppStage!")
				}
			})

			It("returns no error and it won't stage", func() {
				epinioClient, err := usercmd.NewEpinioClient(&settings.Settings{Namespace: "workspace"}, mockClient)
				Expect(err).ToNot(HaveOccurred())

				err = epinioClient.AppRestage("appname")
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})

type mockAPIClient struct {
	mockAppShow         func(namespace string, appName string) (models.App, error)
	mockAppStage        func(req models.StageRequest) (*models.StageResponse, error)
	mockAppLogs         func(namespace, appName, stageID string, follow bool, callback func(tailer.ContainerLogLine)) error
	mockStagingComplete func(namespace string, id string) (models.Response, error)
}

func (m *mockAPIClient) AuthToken() (string, error) {
	return "", nil
}

func (m *mockAPIClient) AppCreate(req models.ApplicationCreateRequest, namespace string) (models.Response, error) {
	return models.Response{}, nil
}

func (m *mockAPIClient) Apps(namespace string) (models.AppList, error) {
	return models.AppList{}, nil
}

func (m *mockAPIClient) AllApps() (models.AppList, error) {
	return models.AppList{}, nil
}

func (m *mockAPIClient) AppShow(namespace string, appName string) (models.App, error) {
	return m.mockAppShow(namespace, appName)
}

func (m *mockAPIClient) AppGetPart(namespace, appName, part, destination string) error {
	return nil
}

func (m *mockAPIClient) AppUpdate(req models.ApplicationUpdateRequest, namespace string, appName string) (models.Response, error) {
	return models.Response{}, nil
}

func (m *mockAPIClient) AppDelete(namespace string, name string) (models.ApplicationDeleteResponse, error) {
	return models.ApplicationDeleteResponse{}, nil
}

func (m *mockAPIClient) AppUpload(namespace string, name string, tarball string) (models.UploadResponse, error) {
	return models.UploadResponse{}, nil
}

func (m *mockAPIClient) AppImportGit(app models.AppRef, gitRef models.GitRef) (*models.ImportGitResponse, error) {
	return nil, nil
}

func (m *mockAPIClient) AppStage(req models.StageRequest) (*models.StageResponse, error) {
	return m.mockAppStage(req)
}

func (m *mockAPIClient) AppDeploy(req models.DeployRequest) (*models.DeployResponse, error) {
	return nil, nil
}

func (m *mockAPIClient) AppLogs(namespace, appName, stageID string, follow bool, callback func(tailer.ContainerLogLine)) error {
	return m.mockAppLogs(namespace, appName, stageID, follow, callback)
}

func (m *mockAPIClient) StagingComplete(namespace string, id string) (models.Response, error) {
	return m.mockStagingComplete(namespace, id)
}

func (m *mockAPIClient) AppRunning(app models.AppRef) (models.Response, error) {
	return models.Response{}, nil
}

func (m *mockAPIClient) AppExec(namespace string, appName, instance string, tty kubectlterm.TTY) error {
	return nil
}

func (m *mockAPIClient) AppPortForward(namespace string, appName, instance string, opts *epinioapi.PortForwardOpts) error {
	return nil
}

func (m *mockAPIClient) AppRestart(namespace string, appName string) error {
	return nil
}

func (m *mockAPIClient) EnvList(namespace string, appName string) (models.EnvVariableMap, error) {
	return models.EnvVariableMap{}, nil
}

func (m *mockAPIClient) EnvSet(req models.EnvVariableMap, namespace string, appName string) (models.Response, error) {
	return models.Response{}, nil
}

func (m *mockAPIClient) EnvShow(namespace string, appName string, envName string) (models.EnvVariable, error) {
	return models.EnvVariable{}, nil
}

func (m *mockAPIClient) EnvUnset(namespace string, appName string, envName string) (models.Response, error) {
	return models.Response{}, nil
}

func (m *mockAPIClient) EnvMatch(namespace string, appName string, prefix string) (models.EnvMatchResponse, error) {
	return models.EnvMatchResponse{}, nil
}

func (m *mockAPIClient) Info() (models.InfoResponse, error) {
	return models.InfoResponse{}, nil
}

func (m *mockAPIClient) NamespaceCreate(req models.NamespaceCreateRequest) (models.Response, error) {
	return models.Response{}, nil
}

func (m *mockAPIClient) NamespaceDelete(namespace string) (models.Response, error) {
	return models.Response{}, nil
}

func (m *mockAPIClient) NamespaceShow(namespace string) (models.Namespace, error) {
	return models.Namespace{}, nil
}

func (m *mockAPIClient) NamespacesMatch(prefix string) (models.NamespacesMatchResponse, error) {
	return models.NamespacesMatchResponse{}, nil
}

func (m *mockAPIClient) Namespaces() (models.NamespaceList, error) {
	return models.NamespaceList{}, nil
}

func (m *mockAPIClient) Configurations(namespace string) (models.ConfigurationResponseList, error) {
	return models.ConfigurationResponseList{}, nil
}

func (m *mockAPIClient) AllConfigurations() (models.ConfigurationResponseList, error) {
	return models.ConfigurationResponseList{}, nil
}

func (m *mockAPIClient) ConfigurationBindingCreate(req models.BindRequest, namespace string, appName string) (models.BindResponse, error) {
	return models.BindResponse{}, nil
}

func (m *mockAPIClient) ConfigurationBindingDelete(namespace string, appName string, serviceName string) (models.Response, error) {
	return models.Response{}, nil
}

func (m *mockAPIClient) ConfigurationDelete(req models.ConfigurationDeleteRequest, namespace string, name string, f epinioapi.ErrorFunc) (models.ConfigurationDeleteResponse, error) {
	return models.ConfigurationDeleteResponse{}, nil
}

func (m *mockAPIClient) ConfigurationCreate(req models.ConfigurationCreateRequest, namespace string) (models.Response, error) {
	return models.Response{}, nil
}

func (m *mockAPIClient) ConfigurationUpdate(req models.ConfigurationUpdateRequest, namespace, name string) (models.Response, error) {
	return models.Response{}, nil
}

func (m *mockAPIClient) ConfigurationShow(namespace string, name string) (models.ConfigurationResponse, error) {
	return models.ConfigurationResponse{}, nil
}

func (m *mockAPIClient) ConfigurationApps(namespace string) (models.ConfigurationAppsResponse, error) {
	return models.ConfigurationAppsResponse{}, nil
}

func (m *mockAPIClient) ServiceCatalog() (*models.ServiceCatalogResponse, error) {
	return nil, nil
}

func (m *mockAPIClient) ServiceCatalogShow(serviceName string) (*models.ServiceCatalogShowResponse, error) {
	return nil, nil
}

func (m *mockAPIClient) AllServices() (*models.ServiceListResponse, error) {
	return nil, nil
}

func (m *mockAPIClient) ServiceShow(req *models.ServiceShowRequest, namespace string) (*models.ServiceShowResponse, error) {
	return nil, nil
}

func (m *mockAPIClient) ServiceDelete(req models.ServiceDeleteRequest, namespace string, name string, f epinioapi.ErrorFunc) (models.ServiceDeleteResponse, error) {
	return models.ServiceDeleteResponse{}, nil
}

func (m *mockAPIClient) ServiceCreate(req *models.ServiceCreateRequest, namespace string) error {
	return nil
}

func (m *mockAPIClient) ServiceBind(req *models.ServiceBindRequest, namespace, releaseName string) error {
	return nil
}

func (m *mockAPIClient) ServiceUnbind(req *models.ServiceUnbindRequest, namespace, releaseName string) error {
	return nil
}

func (m *mockAPIClient) ServiceList(namespace string) (*models.ServiceListResponse, error) {
	return nil, nil
}

func (m *mockAPIClient) ChartList() ([]models.AppChart, error) {
	return []models.AppChart{}, nil
}

func (m *mockAPIClient) ChartShow(name string) (models.AppChart, error) {
	return models.AppChart{}, nil
}

func (m *mockAPIClient) ChartMatch(prefix string) (models.ChartMatchResponse, error) {
	return models.ChartMatchResponse{}, nil
}
