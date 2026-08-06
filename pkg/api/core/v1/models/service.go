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

type Service struct {
	Meta                  Meta               `json:"meta,omitempty"`
	SecretTypes           []string           `json:"secretTypes,omitempty"`
	CatalogService        string             `json:"catalog_service,omitempty"`
	CatalogServiceVersion string             `json:"catalog_service_version,omitempty"`
	Status                ServiceStatus      `json:"status,omitempty"`
	BoundApps             []string           `json:"boundapps"`
	InternalRoutes        []string           `json:"internal_routes,omitempty"`
	Settings              ChartValueSettings `json:"settings,omitempty"`
	Details               map[string]string  `json:"details,omitempty"` // Details from associated configs
}

func (s Service) Namespace() string {
	return s.Meta.Namespace
}

type ServiceStatus string

const (
	ServiceStatusDeployed ServiceStatus = "deployed"
	ServiceStatusNotReady ServiceStatus = "not-ready"
	ServiceStatusUnknown  ServiceStatus = "unknown"
)

func (s ServiceStatus) String() string { return string(s) }

// ServiceList represents a collection of service instances
type ServiceList []Service

// ServiceMatchResponse contains the list of names for matching services
type ServiceMatchResponse struct {
	Names []string `json:"names,omitempty"`
}

// ServiceAppsResponse returns a list of apps per service
type ServiceAppsResponse struct {
	AppsOf map[string]AppList `json:"apps_of,omitempty"`
}

type ServiceCreateRequest struct {
	CatalogService string             `json:"catalog_service,omitempty"`
	Name           string             `json:"name,omitempty"`
	Wait           bool               `json:"wait,omitempty"`
	Settings       ChartValueSettings `json:"settings,omitempty" yaml:"settings,omitempty"`
}

// NOTE: The `Update` and `Replace` requests below serve the same function, the modification and
// redeployment of an existing service with changed custom values. The two endpoint differ in the
// representation of the change and through that which user they are suitable for.
//
// `Update` takes a set of change/remove instructions and applies them to the service. This is
// suitable to the CLI, which has no knowledge of the current state of the service.
//
// `Replace` on the other hand simply provides the entire new set of keys and values to replace the
// current data with. This is suitable to the Web UI which has a local copy of the service state
// available.

// ServiceUpdateRequest represents and contains the data needed to
// update a service instance (add/change, and remove custom value keys)
type ServiceUpdateRequest struct {
	Remove  []string           `json:"remove,omitempty"`
	Set     ChartValueSettings `json:"edit,omitempty"`
	Wait    bool               `json:"wait,omitempty"`
	Restart *bool              `json:"restart,omitempty"`
}

// ServiceReplaceRequest represents and contains the data needed to
// replace a service instance (i.e. the custom value keys)
type ServiceReplaceRequest struct {
	Settings ChartValueSettings `json:"settings,omitempty"`
	Wait     bool               `json:"wait,omitempty"`
	Restart  *bool              `json:"restart,omitempty"`
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

// ServiceBatchBindRequest represents a request to bind multiple services to an application at once
type ServiceBatchBindRequest struct {
	AppName      string   `json:"app_name,omitempty"`
	ServiceNames []string `json:"service_names,omitempty"`
}

// CatalogServices is a list of catalog service elements
type CatalogServices []CatalogService

// CatalogMatchResponse contains the list of names for matching catalog entries
type CatalogMatchResponse struct {
	Names []string `json:"names,omitempty"`
}

// CatalogService mostly matches github.com/epinio/application/api/v1 ServiceSpec
// Reason for existence: Do not expose the internal CRD struct in the API.
type CatalogService struct {
	Meta             MetaLite                `json:"meta,omitempty"`
	SecretTypes      []string                `json:"secret_types,omitempty"`
	Description      string                  `json:"description,omitempty"`
	ShortDescription string                  `json:"short_description,omitempty"`
	HelmChart        string                  `json:"chart,omitempty"`
	ChartVersion     string                  `json:"chart_version,omitempty"`
	ServiceIcon      string                  `json:"service_icon,omitempty"`
	AppVersion       string                  `json:"app_version,omitempty"`
	HelmRepo         HelmRepo                `json:"helm_repo,omitempty"`
	Values           string                  `json:"values,omitempty"`
	Settings         map[string]ChartSetting `json:"settings,omitempty"`
	// BoundServices reports whether at least one provisioned service instance
	// derives from this catalog service. Read-only, computed from live service
	// instances. Named for what actually binds to a catalog service (instances),
	// not apps, which attach to instances rather than the catalog entry.
	BoundServices bool `json:"bound_services,omitempty"`
}

// HelmRepo matches github.com/epinio/application/api/v1 HelmRepo
// Reason for existence: Do not expose the internal CRD struct in the API.
type HelmRepo struct {
	Name string   `json:"name,omitempty"`
	URL  string   `json:"url,omitempty"`
	Auth HelmAuth `json:"-"`
}

// HelmAuth contains the credentials to login into an OCI registry or a private Helm repository
type HelmAuth struct {
	Username string `json:"-"`
	Password string `json:"-"`
}

// HelmRepoRequest mirrors the on-cluster CR HelmRepo, including the Secret
// reference used to resolve credentials. Used by catalog service write
// requests; auth values themselves are never accepted in the request body.
type HelmRepoRequest struct {
	Name   string `json:"name,omitempty"`
	URL    string `json:"url,omitempty"`
	Secret string `json:"secret,omitempty"`
}

// CatalogServiceCreateRequest carries the fields a client supplies when
// creating a catalog service. Name is required and lands in metadata.name;
// the remainder land under spec.
type CatalogServiceCreateRequest struct {
	Name             string                  `json:"name,omitempty"`
	ShortDescription string                  `json:"short_description,omitempty"`
	Description      string                  `json:"description,omitempty"`
	HelmChart        string                  `json:"chart,omitempty"`
	ChartVersion     string                  `json:"chart_version,omitempty"`
	AppVersion       string                  `json:"app_version,omitempty"`
	ServiceIcon      string                  `json:"service_icon,omitempty"`
	Values           string                  `json:"values,omitempty"`
	HelmRepo         HelmRepoRequest         `json:"helm_repo,omitempty"`
	Settings         map[string]ChartSetting `json:"settings,omitempty"`
	SecretTypes      []string                `json:"secret_types,omitempty"`
}

// CatalogServiceUpdateRequest carries optional field updates. Empty string
// fields are ignored — name is taken from the URL, not the body. Settings
// and SecretTypes are replaced when non-nil; pass nil to leave untouched.
type CatalogServiceUpdateRequest struct {
	ShortDescription string                  `json:"short_description,omitempty"`
	Description      string                  `json:"description,omitempty"`
	HelmChart        string                  `json:"chart,omitempty"`
	ChartVersion     string                  `json:"chart_version,omitempty"`
	AppVersion       string                  `json:"app_version,omitempty"`
	ServiceIcon      string                  `json:"service_icon,omitempty"`
	Values           string                  `json:"values,omitempty"`
	HelmRepo         *HelmRepoRequest        `json:"helm_repo,omitempty"`
	Settings         map[string]ChartSetting `json:"settings,omitempty"`
	SecretTypes      []string                `json:"secret_types,omitempty"`
}
