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

package docs

//go:generate swagger generate spec

import "github.com/epinio/epinio/pkg/api/core/v1/models"

// swagger:route GET /catalogservices service ServiceCatalog
// Return all available Epinio Catalog services.
// responses:
//   200: ServiceCatalogResponse

// swagger:parameters ServiceCatalog
type ServiceCatalogParam struct{}

// swagger:response ServiceCatalogResponse
type ServiceCatalogResponse struct {
	// in: body
	Body models.CatalogServices
}

// swagger:route GET /catalogservices/{CatalogService} service ServiceCatalogShow
// Return details of the named Epinio `CatalogService`.
// responses:
//   200: ServiceCatalogShowResponse

// swagger:parameters ServiceCatalogShow
type ServiceCatalogShowParam struct {
	// in: path
	CatalogService string
}

// swagger:response ServiceCatalogShowResponse
type ServiceCatalogShowResponse struct {
	// in: body
	Body models.CatalogService
}

// swagger:route GET /catalogservicesmatches/{Pattern} catalogservice CatalogServiceMatch
// Return list of names for all catalog entries whose name matches the prefix `Pattern`.
// responses:
//   200: CatalogMatchResponse

// swagger:parameters CatalogServiceMatch
type CatalogServiceMatchParam struct {
	// in: path
	Pattern string
}

// swagger:response CatalogMatchResponse
type CatalogMatchResponse struct {
	// in: body
	Body models.CatalogMatchResponse
}

// swagger:route GET /services service AllServices
// Return all the `Services` where the User has authorization.
// responses:
//   200: ServiceListResponse

// swagger:route POST /namespaces/{Namespace}/services service ServiceCreate
// Create a named service of an Epinio catalog service in the `Namespace`.
// responses:
//   200: ServiceCreateResponse

// swagger:parameters ServiceCreate
type ServiceCreateParam struct {
	// in: path
	Namespace string
	// in: body
	Configuration models.ServiceCreateRequest
}

// swagger:response ServiceCreateResponse
type ServiceCreateResponse struct {
	// in: body
	Body models.Response
}

// swagger:route PATCH /namespaces/{Namespace}/services/{Service} service ServiceUpdate
// Update the named `Service` in the `Namespace` as per the instructions in the body
// responses:
//   200: ServiceUpdateResponse

// swagger:parameters ServiceUpdate
type ServiceUpdateParam struct {
	// in: path
	Namespace string
	// in: path
	Service string
	// in: body
	Body models.ServiceUpdateRequest
}

// swagger:response ServiceUpdateResponse
type ServiceUpdateResponse struct {
	// in: body
	Body models.Response
}

// swagger:route PUT /namespaces/{Namespace}/services/{Service} service ServiceReplace
// Replace the named `Service` in the `Namespace` as per the instructions in the body
// responses:
//   200: ServiceReplaceResponse

// swagger:parameters ServiceReplace
type ServiceReplaceParam struct {
	// in: path
	Namespace string
	// in: path
	Service string
	// in: body
	Body models.ServiceReplaceRequest
}

// swagger:response ServiceReplaceResponse
type ServiceReplaceResponse struct {
	// in: body
	Body models.Response
}

// swagger:route GET /namespaces/{Namespace}/services service ServiceList
// Return list of services in the `Namespace`.
// responses:
//   200: ServiceListResponse

// swagger:parameters ServiceList
type ServiceListParam struct {
	// in: path
	Namespace string
}

// swagger:response ServiceListResponse
type ServiceListResponse struct {
	// in: body
	Body models.ServiceList
}

// swagger:route GET /namespaces/{Namespace}/services/{Service} service ServiceShow
// Return details of the named `Service` in the `Namespace`.
// responses:
//   200: ServiceShowResponse

// swagger:response ServiceShowResponse
type ServiceShowResponse struct {
	// in: body
	Body models.Service
}

// swagger:parameters ServiceShow
type ServiceShowParam struct {
	// in: path
	Namespace string
	// in: path
	Service string
}

// swagger:route GET /namespace/{Namespace}/servicesmatches/{Pattern} service ServiceMatch
// Return list of names for all services whose name matches the prefix `Pattern`.
// responses:
//   200: ServiceMatchResponse

// swagger:parameters ServiceMatch
type ServiceMatchParam struct {
	// in: path
	Namespace string
	// in: path
	Pattern string
}

// swagger:response ServiceMatchResponse
type ServiceMatchResponse struct {
	// in: body
	Body models.ServiceMatchResponse
}

// swagger:route DELETE /namespaces/{Namespace}/services/{Service} service ServiceDelete
// Delete the named `Service` in the `Namespace`.
// responses:
//   200: ServiceDeleteResponse

// swagger:route DELETE /namespaces/{Namespace}/services ServiceBatchDelete
// Delete the named `Services` in the `Namespace`.
// responses:
//   200: ServiceDeleteResponse

// swagger:parameters ServiceDelete
type ServiceDeleteParam struct {
	// in: path
	Namespace string
	// in: path
	Service string
	// in: body
	Body models.ServiceDeleteRequest
}

// swagger:parameters ServiceBatchDelete
type ServiceBatchDeleteParam struct {
	// in: path
	Namespace string
	// in: url
	Services []string
	// in: body
	Body models.ServiceDeleteRequest
}

// swagger:response ServiceDeleteResponse
type ServiceDeleteResponse struct {
	// in: body
	Body models.ServiceDeleteResponse
}

// swagger:route GET /namespaces/{Namespace}/serviceapps service ServiceApps
// Return map from services in the `Namespace`, to the apps in the same.
// responses:
//   200: ServiceAppsResponse

// swagger:parameters ServiceApps
type ServiceAppsParam struct {
	// in: path
	Namespace string
}

// swagger:response ServiceAppsResponse
type ServiceAppsResponse struct {
	// in: body
	Body map[string]models.AppList
}

// swagger:route POST /namespaces/{Namespace}/services/{Service}/bind service ServiceBind
// Bind the named `Service` in the `Namespace` to an App.
// responses:
//   200: ServiceBindResponse

// swagger:parameters ServiceBind
type ServiceBindParam struct {
	// in: path
	Namespace string
	// in: path
	Service string
	// in: body
	Configuration models.ServiceBindRequest
}

// swagger:response ServiceBindResponse
type ServiceBindResponse struct {
	// in: body
	Body models.Response
}

// swagger:route POST /namespaces/{Namespace}/services/{Service}/unbind service ServiceUnbind
// Unbind the named `Service` in the `Namespace` from an App.
// responses:
//   200: ServiceUnbindResponse

// swagger:parameters ServiceUnbind
type ServiceUnbindParam struct {
	// in: path
	Namespace string
	// in: path
	Service string
	// in: body
	Configuration models.ServiceUnbindRequest
}

// swagger:response ServiceUnbindResponse
type ServiceUnbindResponse struct {
	// in: body
	Body models.Response
}

// swagger:parameters ServicePortForward
type ServicePortForwardParam struct {
	// in: path
	Namespace string
	// in: path
	Service string
}

// swagger:response ServicePortForwardResponse
type ServicePortForwardResponse struct{}
