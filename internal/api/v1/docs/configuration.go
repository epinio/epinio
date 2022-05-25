package docs

//go:generate swagger generate spec

import "github.com/epinio/epinio/pkg/api/core/v1/models"

// Configurations

// swagger:route DELETE /namespaces/{Namespace}/configurations/{Configuration} configuration ConfigurationDelete
// Delete the named `Configuration` in the `Namespace`.
// responses:
//   200: ConfigurationDeleteResponse

// swagger:parameters ConfigurationDelete
type ConfigurationDeleteParam struct {
	// in: path
	Namespace string
	// in: path
	Configuration string
	// in: body
	Body models.ConfigurationDeleteRequest
}

// swagger:response ConfigurationDeleteResponse
type ConfigurationDeleteResponse struct {
	// in: body
	Body models.ConfigurationDeleteResponse
}

// swagger:route GET /namespaces/{Namespace}/configurationapps configuration ConfigurationApps
// Return map from configurations in the `Namespace`, to the apps in the same.
// responses:
//   200: ConfigurationAppsResponse

// swagger:parameters ConfigurationApps
type ConfigurationAppsParam struct {
	// in: path
	Namespace string
}

// swagger:response ConfigurationAppsResponse
type ConfigurationAppsResponse struct {
	// in: body
	Body map[string]models.AppList
}

// swagger:route GET /namespaces/{Namespace}/configurations configuration Configurations
// Return list of configurations in the `Namespace`.
// responses:
//   200: ConfigurationsResponse

// swagger:parameters Configurations
type ConfigurationsParam struct {
	// in: path
	Namespace string
}

// swagger:response ConfigurationsResponse
type ConfigurationsResponse struct {
	// in: body
	Body models.ConfigurationResponseList
}

// swagger:route GET /namespaces/{Namespace}/configurations/{Configuration} configuration ConfigurationShow
// Return details of the named `Configuration` in the `Namespace`.
// responses:
//   200: ConfigurationShowResponse

// swagger:parameters ConfigurationShow
type ConfigurationShowParam struct {
	// in: path
	Namespace string
	// in: path
	Configuration string
}

// swagger:response ConfigurationShowResponse
type ConfigurationShowResponse struct {
	// in: body
	Body models.ConfigurationResponse
}

// swagger:route GET /namespace/{Namespace}/configurationsmatches/{Pattern} configuration ConfigurationMatch
// Return list of names for all configurations whose name matches the prefix `Pattern`.
// responses:
//   200: ConfigurationMatchResponse

// swagger:parameters ConfigurationMatch
type ConfigurationMatchParam struct {
	// in: path
	Namespace string
	// in: path
	Pattern string
}

// swagger:response ConfigurationMatchResponse
type ConfigurationMatchResponse struct {
	// in: body
	Body models.ConfigurationMatchResponse
}

// swagger:route POST /namespaces/{Namespace}/configurations configuration ConfigurationCreate
// Create the posted new configuration in the `Namespace`.
// responses:
//   200: ConfigurationCreateResponse

// swagger:parameters ConfigurationCreate
type ConfigurationCreateParam struct {
	// in: path
	Namespace string
	// in: body
	Configuration models.ConfigurationCreateRequest
}

// swagger:response ConfigurationCreateResponse
type ConfigurationCreateResponse struct {
	// in: body
	Body models.Response
}

// swagger:route PATCH /namespaces/{Namespace}/configurations/{Configuration} configuration ConfigurationUpdate
// Update the named `Configuration` in the `Namespace` as per the instructions in the body
// responses:
//   200: ConfigurationUpdateResponse

// swagger:parameters ConfigurationUpdate
type ConfigurationUpdateParam struct {
	// in: path
	Namespace string
	// in: path
	Configuration string
	// in: body
	Body models.ConfigurationUpdateRequest
}

// swagger:response ConfigurationUpdateResponse
type ConfigurationUpdateResponse struct {
	// in: body
	Body models.Response
}

// swagger:route PUT /namespaces/{Namespace}/configurations/{Configuration} configuration ConfigurationReplace
// Replace the named `Configuration` in the `Namespace` as per the instructions in the body
// responses:
//   200: ConfigurationReplaceResponse

// swagger:parameters ConfigurationReplace
type ConfigurationReplaceParam struct {
	// in: path
	Namespace string
	// in: path
	Configuration string
	// in: body
	Body models.ConfigurationReplaceRequest
}

// swagger:response ConfigurationReplaceResponse
type ConfigurationReplaceResponse struct {
	// in: body
	Body models.Response
}

// swagger:route GET /configurations configuration AllConfigurations
// Return list of configurations in all namespaces.
// responses:
//   200: ConfigurationsResponse

// swagger:parameters AllConfigurations
type ConfigurationAllConfigurationsParam struct{}

// response: See Configurations.
