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

// Package usercmd provides Epinio CLI commands for users
package usercmd

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
	"runtime"

	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/selfupdater"
	epinioapi "github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	kubectlterm "k8s.io/kubectl/pkg/util/term"

	"github.com/go-logr/logr"
)

// EpinioClient provides functionality for talking to a
// Epinio installation on Kubernetes
type EpinioClient struct {
	Settings *settings.Settings
	Log      logr.Logger
	ui       *termui.UI
	API      APIClient
	Updater  selfupdater.Updater
}

//counterfeiter:generate . APIClient
type APIClient interface {
	AuthToken() (string, error)

	// app
	AppCreate(req models.ApplicationCreateRequest, namespace string) (models.Response, error)
	Apps(namespace string) (models.AppList, error)
	AllApps() (models.AppList, error)
	AppShow(namespace string, appName string) (models.App, error)
	AppUpdate(req models.ApplicationUpdateRequest, namespace string, appName string) (models.Response, error)
	AppDelete(namespace string, names []string) (models.ApplicationDeleteResponse, error)
	AppUpload(namespace string, name string, tarball string) (models.UploadResponse, error)
	AppImportGit(app models.AppRef, gitRef models.GitRef) (*models.ImportGitResponse, error)
	AppStage(req models.StageRequest) (*models.StageResponse, error)
	AppDeploy(req models.DeployRequest) (*models.DeployResponse, error)
	AppLogs(namespace, appName, stageID string, follow bool, callback func(tailer.ContainerLogLine)) error
	StagingComplete(namespace string, id string) (models.Response, error)
	AppRunning(app models.AppRef) (models.Response, error)
	AppExec(ctx context.Context, namespace string, appName, instance string, tty kubectlterm.TTY) error
	AppPortForward(namespace string, appName, instance string, opts *epinioapi.PortForwardOpts) error
	AppRestart(namespace string, appName string) error
	AppGetPart(namespace, appName, part, destinationPath string) error
	AppMatch(namespace, prefix string) (models.AppMatchResponse, error)
	AppValidateCV(namespace string, name string) (models.Response, error)

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
	NamespaceDelete(namespaces []string) (models.Response, error)
	NamespaceShow(namespace string) (models.Namespace, error)
	NamespacesMatch(prefix string) (models.NamespacesMatchResponse, error)
	Namespaces() (models.NamespaceList, error)

	// configurations
	Configurations(namespace string) (models.ConfigurationResponseList, error)
	AllConfigurations() (models.ConfigurationResponseList, error)
	ConfigurationBindingCreate(req models.BindRequest, namespace string, appName string) (models.BindResponse, error)
	ConfigurationBindingDelete(namespace string, appName string, configurationName string) (models.Response, error)
	ConfigurationDelete(req models.ConfigurationDeleteRequest, namespace string, names []string, f epinioapi.ErrorFunc) (models.ConfigurationDeleteResponse, error)
	ConfigurationCreate(req models.ConfigurationCreateRequest, namespace string) (models.Response, error)
	ConfigurationUpdate(req models.ConfigurationUpdateRequest, namespace, name string) (models.Response, error)
	ConfigurationShow(namespace string, name string) (models.ConfigurationResponse, error)
	ConfigurationApps(namespace string) (models.ConfigurationAppsResponse, error)
	ConfigurationMatch(namespace, prefix string) (models.ConfigurationMatchResponse, error)

	// services
	ServiceCatalog() (models.CatalogServices, error)
	ServiceCatalogShow(serviceName string) (*models.CatalogService, error)
	ServiceCatalogMatch(prefix string) (models.CatalogMatchResponse, error)

	AllServices() (models.ServiceList, error)
	ServiceShow(req *models.ServiceShowRequest, namespace string) (*models.Service, error)
	ServiceCreate(req *models.ServiceCreateRequest, namespace string) error
	ServiceBind(req *models.ServiceBindRequest, namespace, name string) error
	ServiceUnbind(req *models.ServiceUnbindRequest, namespace, name string) error
	ServiceDelete(req models.ServiceDeleteRequest, namespace string, names []string, f epinioapi.ErrorFunc) (models.ServiceDeleteResponse, error)
	ServiceList(namespace string) (models.ServiceList, error)
	ServiceMatch(namespace, prefix string) (models.ServiceMatchResponse, error)

	// application charts
	ChartList() ([]models.AppChart, error)
	ChartShow(name string) (models.AppChart, error)
	ChartMatch(prefix string) (models.ChartMatchResponse, error)

	DisableVersionWarning()
	VersionWarningEnabled() bool
}

func New(ctx context.Context) (*EpinioClient, error) {
	cfg, err := settings.Load()
	if err != nil {
		return nil, errors.Wrap(err, "error loading settings")
	}

	apiClient := epinioapi.New(ctx, cfg)

	return NewEpinioClient(cfg, apiClient)
}

func NewEpinioClient(cfg *settings.Settings, apiClient APIClient) (*EpinioClient, error) {
	logger := tracelog.NewLogger().WithName("EpinioClient").V(3)

	log := logger.WithName("NewEpinioClient")
	log.Info("Ingress API", "url", cfg.API)
	log.Info("Settings API", "url", cfg.API)

	updater, err := getUpdater(runtime.GOOS)
	if err != nil {
		return nil, errors.Wrap(err, "getting updater")
	}

	return &EpinioClient{
		API:      apiClient,
		ui:       termui.NewUI(),
		Settings: cfg,
		Log:      logger,
		Updater:  updater,
	}, nil
}

func (cli *EpinioClient) UI() *termui.UI {
	return cli.ui
}

func getUpdater(os string) (selfupdater.Updater, error) {
	var updater selfupdater.Updater
	switch os {
	case "linux", "darwin":
		updater = selfupdater.PosixUpdater{}
	case "windows":
		updater = selfupdater.WindowsUpdater{}
	default:
		return nil, errors.New("unknown operating system")
	}

	return updater, nil
}
