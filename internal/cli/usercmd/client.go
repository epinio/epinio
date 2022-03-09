// Package usercmd provides Epinio CLI commands for users
package usercmd

import (
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/settings"
	epinioapi "github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	kubectlterm "k8s.io/kubectl/pkg/util/term"

	"github.com/go-logr/logr"
)

var epinioClientMemo *epinioapi.Client

// EpinioClient provides functionality for talking to a
// Epinio installation on Kubernetes
type EpinioClient struct {
	Settings *settings.Settings
	Log      logr.Logger
	ui       *termui.UI
	API      ApiClient
}

type ApiClient interface {
	AuthToken() (string, error)
	// app
	AppCreate(req models.ApplicationCreateRequest, namespace string) (models.Response, error)
	Apps(namespace string) (models.AppList, error)
	AllApps() (models.AppList, error)
	AppShow(namespace string, appName string) (models.App, error)
	AppUpdate(req models.ApplicationUpdateRequest, namespace string, appName string) (models.Response, error)
	AppDelete(namespace string, name string) (models.ApplicationDeleteResponse, error)
	AppUpload(namespace string, name string, tarball string) (models.UploadResponse, error)
	AppImportGit(app models.AppRef, gitRef models.GitRef) (*models.ImportGitResponse, error)
	AppStage(req models.StageRequest) (*models.StageResponse, error)
	AppDeploy(req models.DeployRequest) (*models.DeployResponse, error)
	StagingComplete(namespace string, id string) (models.Response, error)
	AppRunning(app models.AppRef) (models.Response, error)
	AppExec(namespace string, appName, instance string, tty kubectlterm.TTY) error
	AppPortForward(namespace string, appName, instance string, opts *epinioapi.PortForwardOpts) error
	AppRestart(namespace string, appName string) error
	// env
	EnvList(namespace string, appName string) (models.EnvVariableMap, error)
	EnvSet(req models.EnvVariableMap, namespace string, appName string) (models.Response, error)
	EnvShow(namespace string, appName string, envName string) (models.EnvVariable, error)
	EnvUnset(namespace string, appName string, envName string) (models.Response, error)
	EnvMatch(namespace string, appName string, prefix string) (models.EnvMatchResponse, error)
	// info
	Info() (models.InfoResponse, error)
	// namespaces
	NamespaceCreate(req models.NamespaceCreateRequest) (models.Response, error)
	NamespaceDelete(namespace string) (models.Response, error)
	NamespaceShow(namespace string) (models.Namespace, error)
	NamespacesMatch(prefix string) (models.NamespacesMatchResponse, error)
	Namespaces() (models.NamespaceList, error)
	// services
	Services(namespace string) (models.ServiceResponseList, error)
	AllServices() (models.ServiceResponseList, error)
	ServiceBindingCreate(req models.BindRequest, namespace string, appName string) (models.BindResponse, error)
	ServiceBindingDelete(namespace string, appName string, serviceName string) (models.Response, error)
	ServiceDelete(req models.ServiceDeleteRequest, namespace string, name string, f epinioapi.ErrorFunc) (models.ServiceDeleteResponse, error)
	ServiceCreate(req models.ServiceCreateRequest, namespace string) (models.Response, error)
	ServiceUpdate(req models.ServiceUpdateRequest, namespace, name string) (models.Response, error)
	ServiceShow(namespace string, name string) (models.ServiceResponse, error)
	ServiceApps(namespace string) (models.ServiceAppsResponse, error)
}

func New() (*EpinioClient, error) {
	cfg, err := settings.Load()
	if err != nil {
		return nil, errors.Wrap(err, "error loading settings")
	}

	apiClient, err := NewEpinioAPIClient(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "error getting Epinio API client")
	}

	return NewEpinioClient(cfg, apiClient)
}

func NewEpinioClient(cfg *settings.Settings, apiClient ApiClient) (*EpinioClient, error) {
	ui := termui.NewUI()
	logger := tracelog.NewLogger().WithName("EpinioClient").V(3)

	log := logger.WithName("NewEpinioClient")
	log.Info("Ingress API", "url", cfg.API)
	log.Info("Settings API", "url", cfg.API)

	epinioClient := &EpinioClient{
		API:      apiClient,
		ui:       ui,
		Settings: cfg,
		Log:      logger,
	}
	return epinioClient, nil
}

func NewEpinioAPIClient(cfg *settings.Settings) (*epinioapi.Client, error) {
	log := tracelog.NewLogger().WithName("EpinioApiClient").V(3)
	defer func() {
		if epinioClientMemo != nil {
			log.Info("return", "api", epinioClientMemo.URL, "wss", epinioClientMemo.WsURL)
			return
		}
		log.Info("return")
	}()

	// Check for information cached in memory, and return if such is found
	if epinioClientMemo != nil {
		log.Info("cached in memory")
		return epinioClientMemo, nil
	}

	// Check for information cached in the Epinio settings,
	// and return if such is found. Cache into memory as well.
	log.Info("query settings")

	if cfg.API != "" && cfg.WSS != "" {
		log.Info("cached in settings")

		epinioClient := epinioapi.New(log, cfg.API, cfg.WSS, cfg.User, cfg.Password)
		epinioClientMemo = epinioClient

		return epinioClient, nil
	}

	return nil, errors.New("Epinio no longer queries the cluster, please run epinio settings update or ask your operator for help")
}
