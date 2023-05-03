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

// Package models contains the types (mostly structures) encapsulating
// the API requests and reponses used by the communication between
// epinio client and APIserver.
// Not all of them unfortunately, the simpler ones are coded directly.
// TODO: Give even the most simple requests and responses properly
// named types.
package models

import (
	"fmt"

	"github.com/epinio/epinio/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	helmrelease "helm.sh/helm/v3/pkg/release"
)

// Note: Meta is an extension of `ConfigurationRef` later in the file. Users of
// `ConfigurationRef` should be incrementally switched over to `Meta`.

// Meta holds the information needed to reference a resource (name and namespace), plus
// general descriptive things (creation timestamp).
type Meta struct {
	Name      string      `json:"name"`
	Namespace string      `json:"namespace,omitempty"`
	CreatedAt metav1.Time `json:"createdAt,omitempty"`
}

// MetaLite holds the information needed to reference a non-namespaced resource (name),
// plus general descriptive things (creation timestamp).
type MetaLite struct {
	Name      string      `json:"name"`
	CreatedAt metav1.Time `json:"createdAt,omitempty"`
}

type Response struct {
	Status string `json:"status"`
}

var ResponseOK = Response{"ok"}

type Request struct {
}

// ConfigurationRef references a Configuration by name and namespace
type ConfigurationRef struct {
	Meta
}

// ConfigurationResponse represents the data of a single configuration instance
type ConfigurationResponse struct {
	Meta          ConfigurationRef          `json:"meta"`
	Configuration ConfigurationShowResponse `json:"configuration"`
}

// ConfigurationResponseList represents a collection of configuration instance
type ConfigurationResponseList []ConfigurationResponse

// ConfigurationCreateRequest represents and contains the data needed to
// create a configuration instance
type ConfigurationCreateRequest struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

// ConfigurationUpdateRequest represents and contains the data needed to
// update a configuration instance (add/change, and remove keys)
type ConfigurationUpdateRequest struct {
	Remove []string          `json:"remove,omitempty"`
	Set    map[string]string `json:"edit,omitempty"`
}

// ConfigurationReplaceRequest represents and contains the data needed to
// replace a configuration instance
type ConfigurationReplaceRequest map[string]string

// ConfigurationDeleteRequest represents and contains the data needed to delete a configuration
type ConfigurationDeleteRequest struct {
	Unbind bool `json:"unbind"`
}

// ConfigurationDeleteResponse represents the server's response to a successful configuration deletion
type ConfigurationDeleteResponse struct {
	BoundApps []string `json:"boundapps"`
}

// BindRequest represents and contains the data needed to bind configurations to an application.
type BindRequest struct {
	Names []string `json:"names"`
}

// BindResponse represents the server's response to the successful binding of configurations to
// an application.
type BindResponse struct {
	WasBound []string `json:"wasbound"`
}

// ApplicationManifest represents and contains the data of an application's manifest file,
// plus some auxiliary data never (un)marshalled. Namely, the file's location, and origin
// type tag.
type ApplicationManifest struct {
	ApplicationCreateRequest `yaml:",inline"`
	Self                     string            `yaml:"-"` // Hidden from yaml. The file's location.
	Origin                   ApplicationOrigin `yaml:"origin,omitempty"`
	Staging                  ApplicationStage  `yaml:"staging,omitempty"`
	Namespace                string            `yaml:"namespace,omitempty"`
}

// ApplicationStage is the part of the manifest holding information
// relevant to staging the application's sources. This is, currently,
// only the reference to the Paketo builder image to use.
type ApplicationStage struct {
	Builder string `yaml:"builder,omitempty" json:"builder,omitempty"`
}

// ApplicationOrigin is the part of the manifest describing the origin of the application
// (sources). At most one of the fields may be specified / not empty.
type ApplicationOrigin struct {
	// Hidden from yaml. Type tag to simplify struct usage.
	// Note: we cannot hide this property from the JSON since it's used to unmarshal correctly the result of the Apps endpoint
	// @see failling test here: https://github.com/epinio/epinio/runs/4935898437?check_suite_focus=true
	// We should probably expose a more meaningful value instead of this "Kind" int
	Kind      int     `yaml:"-"`
	Container string  `yaml:"container,omitempty" json:"container,omitempty"`
	Git       *GitRef `yaml:"git,omitempty"       json:"git,omitempty"`
	Path      string  `yaml:"path,omitempty"      json:"path,omitempty"`
	Archive   bool    `yaml:"archive,omitempty"   json:"archive,omitempty"`
}

// manifest origin codes for `Kind`.
const (
	OriginNone = iota
	OriginPath
	OriginGit
	OriginContainer
)

func (o *ApplicationOrigin) String() string {
	switch o.Kind {
	case OriginPath:
		if o.Archive {
			return helpers.AbsPath(o.Path) + " (archive)"
		}
		return helpers.AbsPath(o.Path)
	case OriginGit:
		if o.Git.Revision == "" {
			if o.Git.Branch != "" {
				return fmt.Sprintf("%s on %s", o.Git.URL, o.Git.Branch)
			}
			return o.Git.URL
		}
		if o.Git.Branch != "" {
			return fmt.Sprintf("%s @ %s (on %s)", o.Git.URL, o.Git.Revision, o.Git.Branch)
		}
		return fmt.Sprintf("%s @ %s", o.Git.URL, o.Git.Revision)
	case OriginContainer:
		return o.Container
	default:
		// Nothing
	}
	return "<<undefined>>"
}

// ApplicationCreateRequest represents and contains the data needed to
// create an application (at rest), possibly with presets (configurations)
type ApplicationCreateRequest struct {
	Name          string                   `json:"name"          yaml:"name"`
	Configuration ApplicationUpdateRequest `json:"configuration" yaml:"configuration,omitempty"`
}

// ApplicationUpdateRequest represents and contains the data needed to update
// an application. Specifically to modify the number of replicas to
// run, and the configurations bound to it.
// Note: Instances is a pointer to give us a nil value separate from
// actual integers, as means of communicating `default`/`no change`.
type ApplicationUpdateRequest struct {
	Instances      *int32         `json:"instances"          yaml:"instances,omitempty"`
	Configurations []string       `json:"configurations"     yaml:"configurations,omitempty"`
	Environment    EnvVariableMap `json:"environment"        yaml:"environment,omitempty"`
	Routes         []string       `json:"routes"             yaml:"routes,omitempty"`
	AppChart       string         `json:"appchart,omitempty" yaml:"appchart,omitempty"`
	Settings       AppSettings    `json:"settings,omitempty" yaml:"settings,omitempty"`
}

type ImportGitResponse struct {
	BlobUID string `json:"blobuid,omitempty"`
}

// UploadRequest is a multipart form

// UploadResponse represents the server's response to a successful app sources upload
type UploadResponse struct {
	BlobUID string `json:"blobuid,omitempty"`
}

// StageRequest represents and contains the data needed to stage an application
type StageRequest struct {
	App          AppRef `json:"app,omitempty"`
	BlobUID      string `json:"blobuid,omitempty"`
	BuilderImage string `json:"builderimage,omitempty"`
}

// StageResponse represents the server's response to a successful app staging
type StageResponse struct {
	Stage    StageRef `json:"stage,omitempty"`
	ImageURL string   `json:"image,omitempty"`
}

// DeployRequest represents and contains the data needed to deploy an application
// Note that the overall application configuration (instances, configurations, EVs) is
// already known server side, through AppCreate/AppUpdate requests.
// This request not only comes with the image to deploy, but also the
// information where the sources of that image came from.
type DeployRequest struct {
	App      AppRef            `json:"app,omitempty"`
	Stage    StageRef          `json:"stage,omitempty"`
	ImageURL string            `json:"image,omitempty"`
	Origin   ApplicationOrigin `json:"origin,omitempty"`
}

// DeployResponse represents the server's response to a successful app deployment
type DeployResponse struct {
	Routes []string `json:"routes,omitempty"`
}

// ApplicationDeleteResponse represents the server's response to a successful app deletion
type ApplicationDeleteResponse struct {
	UnboundConfigurations []string `json:"unboundconfigurations"`
}

// EnvMatchResponse contains the list of names for matching env variables
type EnvMatchResponse struct {
	Names []string `json:"names,omitempty"`
}

// ConfigurationShowResponse contains details about a configuration
type ConfigurationShowResponse struct {
	Username  string            `json:"user"`               // Name of user creating it
	Details   map[string]string `json:"details,omitempty"`  // Main information, key/value map
	BoundApps []string          `json:"boundapps"`          // Names of the apps using it
	Type      string            `json:"type,omitempty"`     // User or service-created configuration
	Origin    string            `json:"origin,omitempty"`   // Name of service it came from, if any
	Siblings  []string          `json:"siblings,omitempty"` // Name of other configs from same service, if any
}

// ConfigurationMatchResponse contains the list of names for matching configurations
type ConfigurationMatchResponse struct {
	Names []string `json:"names,omitempty"`
}

// InfoResponse contains information about Epinio and its components
type InfoResponse struct {
	Version             string `json:"version,omitempty"`
	KubeVersion         string `json:"kube_version,omitempty"`
	Platform            string `json:"platform,omitempty"`
	DefaultBuilderImage string `json:"default_builder_image,omitempty"`
}

// AuthTokenResponse contains an auth token
type AuthTokenResponse struct {
	Token string `json:"token,omitempty"`
}

// NamespaceCreateRequest contains the name of the namespace that should be created
type NamespaceCreateRequest struct {
	Name string `json:"name,omitempty"`
}

// NamespacesMatchResponse contains the list of names for matching namespaces
type NamespacesMatchResponse struct {
	Names []string `json:"names,omitempty"`
}

// ConfigurationAppsResponse returns a list of apps per configuration
type ConfigurationAppsResponse struct {
	AppsOf map[string]AppList `json:"apps_of,omitempty"`
}

// ServiceAppsResponse returns a list of apps per service
type ServiceAppsResponse struct {
	AppsOf map[string]AppList `json:"apps_of,omitempty"`
}

// CatalogServices is a list of catalog service elements
type CatalogServices []CatalogService

// CatalogMatchResponse contains the list of names for matching catalog entries
type CatalogMatchResponse struct {
	Names []string `json:"names,omitempty"`
}

type ServiceCreateRequest struct {
	CatalogService string `json:"catalog_service,omitempty"`
	Name           string `json:"name,omitempty"`
	Wait           bool   `json:"wait,omitempty"`
}

// CatalogService mostly matches github.com/epinio/application/api/v1 ServiceSpec
// Reason for existence: Do not expose the internal CRD struct in the API.
type CatalogService struct {
	Meta             MetaLite `json:"meta,omitempty"`
	SecretTypes      []string `json:"secretTypes,omitempty"`
	Description      string   `json:"description,omitempty"`
	ShortDescription string   `json:"short_description,omitempty"`
	HelmChart        string   `json:"chart,omitempty"`
	ChartVersion     string   `json:"chartVersion,omitempty"`
	ServiceIcon      string   `json:"serviceIcon,omitempty"`
	AppVersion       string   `json:"appVersion,omitempty"`
	HelmRepo         HelmRepo `json:"helm_repo,omitempty"`
	Values           string   `json:"values,omitempty"`
}

// HelmRepo matches github.com/epinio/application/api/v1 HelmRepo
// Reason for existence: Do not expose the internal CRD struct in the API.
type HelmRepo struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

// ServiceDeleteRequest represents and contains the data needed to delete a service
type ServiceDeleteRequest struct {
	Unbind bool `json:"unbind"`
}

// ServiceDeleteResponse represents the server's response to a successful service deletion
type ServiceDeleteResponse struct {
	BoundApps []string `json:"boundapps"`
}

type ServiceBindRequest struct {
	AppName string `json:"app_name,omitempty"`
}

type ServiceUnbindRequest struct {
	AppName string `json:"app_name,omitempty"`
}

type ServiceShowRequest struct {
	Name string `json:"name,omitempty"`
}

type Service struct {
	Meta                    Meta          `json:"meta,omitempty"`
	SecretTypes             []string      `json:"secretTypes,omitempty"`
	CatalogService          string        `json:"catalog_service,omitempty"`
	CatalogServiceVersion   string        `json:"catalog_service_version,omitempty"`
	Status                  ServiceStatus `json:"status,omitempty"`
	BoundApps               []string      `json:"boundapps"`
	ManagedByHelmController bool          `json:"hcmanaged"`
	InternalRoutes          []string      `json:"internal_routes,omitempty"`
}

func (s Service) Namespace() string {
	return s.Meta.Namespace
}

type ServiceStatus string

// ServiceList represents a collection of service instances
type ServiceList []Service

// ServiceMatchResponse contains the list of names for matching services
type ServiceMatchResponse struct {
	Names []string `json:"names,omitempty"`
}

const (
	ServiceStatusDeployed ServiceStatus = "deployed"
	ServiceStatusNotReady ServiceStatus = "not-ready"
	ServiceStatusUnknown  ServiceStatus = "unknown"
)

func NewServiceStatusFromHelmRelease(status helmrelease.Status) ServiceStatus {
	switch status {
	case helmrelease.StatusDeployed:
		return ServiceStatusDeployed
	default:
		return ServiceStatusNotReady
	}
}

func (s ServiceStatus) String() string { return string(s) }

// AppChart nearly matches github.com/epinio/application/api/v1 AppChartSpec
// Reason for existence: Do not expose the internal CRD struct in the API.
//
// Differences:
//   - Field `Values` is not made public here. It contains server-internal information the
//     user has no need for.
type AppChart struct {
	Meta             MetaLite                   `json:"meta,omitempty"`
	Description      string                     `json:"description,omitempty"`
	ShortDescription string                     `json:"short_description,omitempty"`
	HelmChart        string                     `json:"helm_chart,omitempty"`
	HelmRepo         string                     `json:"helm_repo,omitempty"`
	Settings         map[string]AppChartSetting `json:"settings,omitempty"`
}

type AppChartFull struct {
	AppChart
	Values map[string]string
}

// AppChartSetting matches github.com/epinio/application/api/v1 AppChartSettings
// Reason for existence: Do not expose the internal CRD struct in the API.
type AppChartSetting struct {
	// Type of the setting (string, bool, number, or integer)
	Type string `json:"type"`

	// Minimal allowed value, for number, integer
	Minimum string `json:"minimum,omitempty"`

	// Maximal allowed value, for number, integer
	Maximum string `json:"maximum,omitempty"`

	// Enumeration of allowed values, for types string, number, integer
	Enum []string `json:"enum,omitempty"`

	// Presence of an enum for number and integer overrides the min/max
	// specifications
}

// AppChartList is a collection of app charts
type AppChartList []AppChart

// ChartMatchResponse contains the list of names for matching application charts
type ChartMatchResponse struct {
	Names []string `json:"names,omitempty"`
}
