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

package models

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/epinio/epinio/internal/names"
)

const (
	EpinioStageIDPrevious   = "epinio.io/previous-stage-id"
	EpinioStageIDLabel      = "epinio.io/stage-id"
	EpinioStageBlobUIDLabel = "epinio.io/blob-uid"

	EpinioCreatedByAnnotation = "epinio.io/created-by"

	ApplicationCreated = "created"
	ApplicationStaging = "staging"
	ApplicationRunning = "running"
	ApplicationError   = "error"

	ApplicationStagingActive = "active"
	ApplicationStagingDone   = "done"
	ApplicationStagingFailed = "failed"
)

type GitProvider string

const (
	ProviderGit              = GitProvider("git")
	ProviderGithub           = GitProvider("github")
	ProviderGithubEnterprise = GitProvider("github_enterprise")
	ProviderGitlab           = GitProvider("gitlab")
	ProviderGitlabEnterprise = GitProvider("gitlab_enterprise")
	ProviderUnknown          = GitProvider("unknown")
)

var ValidProviders = []GitProvider{
	ProviderGit,
	ProviderGithub,
	ProviderGithubEnterprise,
	ProviderGitlab,
	ProviderGitlabEnterprise,
}

func GitProviderFromString(provider string) (GitProvider, error) {
	for _, candidate := range ValidProviders {
		if string(candidate) == provider {
			return candidate, nil
		}
	}
	return ProviderUnknown, errors.New("unknown provider")
}

func (p GitProvider) ValidateURL(gitURL string) error {
	// check provider URL
	u, err := url.Parse(gitURL)
	if err != nil {
		return fmt.Errorf("parsing git url `%s`", gitURL)
	}

	// The only assumption that we can do is that if the host is known (github or gitlab) then we know the provider,
	// otherwise we cannot be sure about it, and we need to trust the user.
	if (u.Host == "github.com" && p != ProviderGithub) ||
		(u.Host == "gitlab.com" && p != ProviderGitlab) {
		return fmt.Errorf("git url and provider mismatch `%s - %s`", gitURL, p)
	}

	return nil
}

type ApplicationStatus string
type ApplicationStagingStatus string

type GitRef struct {
	Revision string      `json:"revision,omitempty" yaml:"revision,omitempty"`
	URL      string      `json:"repository"         yaml:"url,omitempty"`
	Provider GitProvider `json:"provider,omitempty" yaml:"provider,omitempty"`
	Branch   string      `json:"branch,omitempty"   yaml:"branch,omitempty"`
}

// App has all the application's properties, for at rest (Configuration), and active (Workload).
// The main structure has identifying information.
// It is used in the CLI and API responses.
type App struct {
	Meta          AppRef                   `json:"meta"`
	Configuration ApplicationConfiguration `json:"configuration"`
	Origin        ApplicationOrigin        `json:"origin"`
	Workload      *AppDeployment           `json:"deployment,omitempty"`
	Staging       ApplicationStage         `json:"staging,omitempty"`
	StagingStatus ApplicationStagingStatus `json:"stagingstatus"`
	Status        ApplicationStatus        `json:"status"`
	StatusMessage string                   `json:"statusmessage"`
	StageID       string                   `json:"stage_id,omitempty"` // staging id, last run
	ImageURL      string                   `json:"image_url"`
}

type PodInfo struct {
	Name        string `json:"name"`
	MetricsOk   bool   `json:"metricsOk"`
	MemoryBytes int64  `json:"memoryBytes"`
	MilliCPUs   int64  `json:"millicpus"`
	CreatedAt   string `json:"createdAt,omitempty"`
	Restarts    int32  `json:"restarts"`
	Ready       bool   `json:"ready"`
}

// AppDeployment contains all the information specific to an active
// application, i.e. one with a deployment in the cluster.
type AppDeployment struct {
	// TODO: Readiness and Liveness fields?
	Name            string              `json:"name,omitempty"`
	Active          bool                `json:"active,omitempty"` // app is > 0 replicas
	CreatedAt       string              `json:"createdAt,omitempty"`
	DesiredReplicas int32               `json:"desiredreplicas"`
	ReadyReplicas   int32               `json:"readyreplicas"`
	Replicas        map[string]*PodInfo `json:"replicas"`
	Username        string              `json:"username,omitempty"` // app creator
	StageID         string              `json:"stage_id,omitempty"` // staging id, running app
	Status          string              `json:"status,omitempty"`   // app replica status
	Routes          []string            `json:"routes,omitempty"`   // app routes
}

// AppMatchResponse contains the list of names for matching apps
type AppMatchResponse struct {
	Names []string `json:"names,omitempty"`
}

// NewApp returns a new app for name and namespace
func NewApp(name string, namespace string) *App {
	return &App{
		Meta: AppRef{
			Meta{
				Name:      name,
				Namespace: namespace,
			},
		},
	}
}

// AppRef returns a reference to the app (name, namespace)
func (a App) Namespace() string {
	return a.Meta.Namespace
}

// AppRef returns a reference to the app (name, namespace)
func (a *App) AppRef() AppRef {
	return a.Meta
}

// AppList is a collection of app references
type AppList []App

// Implement the Sort interface for application slices

// Len (Sort interface) returns the length of the AppList
func (al AppList) Len() int {
	return len(al)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the AppList
func (al AppList) Swap(i, j int) {
	al[i], al[j] = al[j], al[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the AppList and returns true if the condition holds, and
// else false.
func (al AppList) Less(i, j int) bool {
	return (al[i].Meta.Namespace < al[j].Meta.Namespace) ||
		((al[i].Meta.Namespace == al[j].Meta.Namespace) &&
			(al[i].Meta.Name < al[j].Meta.Name))
}

// AppRef references an App by name and namespace
type AppRef struct {
	Meta
}

// NewAppRef returns a new reference to an app
func NewAppRef(name string, namespace string) AppRef {
	return AppRef{
		Meta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// App returns a fresh app model for the reference
func (ar *AppRef) App() *App {
	return NewApp(ar.Name, ar.Namespace)
}

// MakeEnvSecretName returns the name of the kube secret holding the
// environment variables of the referenced application
func (ar *AppRef) MakeEnvSecretName() string {
	// TODO: This needs tests for env operations on an app with a long name
	return names.GenerateResourceName(ar.Name + "-env")
}

// MakeConfigurationSecretName returns the name of the kube secret holding the
// bound configurations of the referenced application
func (ar *AppRef) MakeConfigurationSecretName() string {
	// TODO: This needs tests for configuration operations on an app with a long name
	return names.GenerateResourceName(ar.Name + "-config")
}

// MakeServiceSecretName returns the name of the kube secret holding the bound
// services of the referenced application
func (ar *AppRef) MakeServiceSecretName() string {
	// TODO: This needs tests for service operations on an app with a long name
	return names.GenerateResourceName(ar.Name + "-svc")
}

// MakeScaleSecretName returns the name of the kube secret holding the number
// of desired instances for referenced application
func (ar *AppRef) MakeScaleSecretName() string {
	// TODO: This needs tests for configuration operations on an app with a long name
	return names.GenerateResourceName(ar.Name + "-scale")
}

// MakePVCName returns the name of the kube pvc to use with/for the referenced application.
func (ar *AppRef) MakePVCName() string {
	return names.GenerateResourceName(ar.Namespace, ar.Name)
}

// StageRef references a staging run by ID, currently randomly generated
// for each POST to the staging endpoint
type StageRef struct {
	ID string `json:"id,omitempty"`
}

// NewStage returns a new reference to a staging run
func NewStage(id string) StageRef {
	return StageRef{id}
}

// ImageRef references an upload
type ImageRef struct {
	ID string `json:"id,omitempty"`
}

// NewImage returns a new image ref for the given ID
func NewImage(id string) ImageRef {
	return ImageRef{id}
}
