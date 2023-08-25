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
	CatalogService string             `json:"catalog_service,omitempty"`
	Name           string             `json:"name,omitempty"`
	Wait           bool               `json:"wait,omitempty"`
	Settings       ChartValueSettings `json:"settings,omitempty" yaml:"settings,omitempty"`
}

// CatalogService mostly matches github.com/epinio/application/api/v1 ServiceSpec
// Reason for existence: Do not expose the internal CRD struct in the API.
type CatalogService struct {
	Meta             MetaLite                `json:"meta,omitempty"`
	SecretTypes      []string                `json:"secretTypes,omitempty"`
	Description      string                  `json:"description,omitempty"`
	ShortDescription string                  `json:"short_description,omitempty"`
	HelmChart        string                  `json:"chart,omitempty"`
	ChartVersion     string                  `json:"chartVersion,omitempty"`
	ServiceIcon      string                  `json:"serviceIcon,omitempty"`
	AppVersion       string                  `json:"appVersion,omitempty"`
	HelmRepo         HelmRepo                `json:"helm_repo,omitempty"`
	Values           string                  `json:"values,omitempty"`
	Settings         map[string]ChartSetting `json:"settings,omitempty"`
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

func (s ServiceStatus) String() string { return string(s) }
