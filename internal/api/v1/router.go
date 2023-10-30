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

// Package v1 is the implementation of Epinio's API v1
// It has the router and controllers (handler funcs) for the API server.
package v1

import (
	"fmt"
	"reflect"
	"runtime"

	"github.com/gin-gonic/gin"

	"github.com/epinio/epinio/helpers/routes"
	"github.com/epinio/epinio/internal/api/v1/appchart"
	"github.com/epinio/epinio/internal/api/v1/application"
	"github.com/epinio/epinio/internal/api/v1/configuration"
	"github.com/epinio/epinio/internal/api/v1/configurationbinding"
	"github.com/epinio/epinio/internal/api/v1/env"
	"github.com/epinio/epinio/internal/api/v1/exportregistry"
	"github.com/epinio/epinio/internal/api/v1/gitconfig"
	"github.com/epinio/epinio/internal/api/v1/gitproxy"
	"github.com/epinio/epinio/internal/api/v1/namespace"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/api/v1/service"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/pkg/api/core/v1/errors"
)

const (
	// Root is the url path prefix for all API endpoints.
	Root = "/api/v1"
	// WsRoot is the url path prefix for all websocket API endpoints.
	WsRoot = "/wapi/v1"
)

// APIActionFunc is matched by all actions. Actions can return a list of errors.
// The "Status" of the first error in the list becomes the response Status Code.
type APIActionFunc func(c *gin.Context) errors.APIErrors

// ErrorHandler exposes the internal errorhandler for use by the server without having to edit the
// router definitions.
func ErrorHandler(action APIActionFunc) gin.HandlerFunc {
	return errorHandler(action)
}

func funcName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func errorHandler(action APIActionFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if errors := action(c); errors != nil {
			requestctx.Logger(c.Request.Context()).Info(
				"responding with json error response",
				"action", funcName(action),
				"errors", errors,
			)
			response.Error(c, errors)
		}
	}
}

func get(path string, h gin.HandlerFunc) routes.Route {
	return routes.NewRoute("GET", path, h)
}

func post(path string, h gin.HandlerFunc) routes.Route {
	return routes.NewRoute("POST", path, h)
}

func delete(path string, h gin.HandlerFunc) routes.Route {
	return routes.NewRoute("DELETE", path, h)
}

func patch(path string, h gin.HandlerFunc) routes.Route {
	return routes.NewRoute("PATCH", path, h)
}

func put(path string, h gin.HandlerFunc) routes.Route {
	return routes.NewRoute("PUT", path, h)
}

// AdminRoutes is the list of restricted routes, only accessible by admins
var AdminRoutes map[string]struct{} = map[string]struct{}{}

var Routes = routes.NamedRoutes{
	"AuthToken": get("/authtoken", errorHandler(AuthToken)),

	// app controller files see application/*.go

	"AllApps":         get("/applications", errorHandler(application.FullIndex)),
	"Apps":            get("/namespaces/:namespace/applications", errorHandler(application.Index)),
	"AppCreate":       post("/namespaces/:namespace/applications", errorHandler(application.Create)),
	"AppShow":         get("/namespaces/:namespace/applications/:app", errorHandler(application.Show)),
	"StagingComplete": get("/namespaces/:namespace/staging/:stage_id/complete", errorHandler(application.Staged)), // See stage.go
	"AppDelete":       delete("/namespaces/:namespace/applications/:app", errorHandler(application.Delete)),
	"AppBatchDelete":  delete("/namespaces/:namespace/applications", errorHandler(application.Delete)),
	"AppDeploy":       post("/namespaces/:namespace/applications/:app/deploy", errorHandler(application.Deploy)),
	"AppImportGit":    post("/namespaces/:namespace/applications/:app/import-git", errorHandler(application.ImportGit)),
	"AppPart":         get("/namespaces/:namespace/applications/:app/part/:part", errorHandler(application.GetPart)),
	"AppRestart":      post("/namespaces/:namespace/applications/:app/restart", errorHandler(application.Restart)),
	"AppRunning":      get("/namespaces/:namespace/applications/:app/running", errorHandler(application.Running)),
	"AppStage":        post("/namespaces/:namespace/applications/:app/stage", errorHandler(application.Stage)), // See stage.go
	"AppUpdate":       patch("/namespaces/:namespace/applications/:app", errorHandler(application.Update)),
	"AppUpload":       post("/namespaces/:namespace/applications/:app/store", errorHandler(application.Upload)), // See upload.go
	"AppValidateCV":   get("/namespaces/:namespace/applications/:app/validate-cv", errorHandler(application.ValidateChartValues)),
	"AppExport":       post("/namespaces/:namespace/applications/:app/export", errorHandler(application.ExportToRegistry)),

	"AppMatch":  get("/namespaces/:namespace/appsmatches/:pattern", errorHandler(application.Match)),
	"AppMatch0": get("/namespaces/:namespace/appsmatches", errorHandler(application.Match)),

	// See env.go
	"EnvList": get("/namespaces/:namespace/applications/:app/environment", errorHandler(env.Index)),

	// Note, the second registration catches calls with an empty pattern!
	"EnvMatch":  get("/namespaces/:namespace/applications/:app/environmentmatch/:pattern", errorHandler(env.Match)),
	"EnvMatch0": get("/namespaces/:namespace/applications/:app/environmentmatch", errorHandler(env.Match)),

	"EnvSet":   post("/namespaces/:namespace/applications/:app/environment", errorHandler(env.Set)),
	"EnvShow":  get("/namespaces/:namespace/applications/:app/environment/:env", errorHandler(env.Show)),
	"EnvUnset": delete("/namespaces/:namespace/applications/:app/environment/:env", errorHandler(env.Unset)),

	// Bind and unbind configurations to/from applications, by means of configurationbindings in applications
	"ConfigurationBindingCreate": post("/namespaces/:namespace/applications/:app/configurationbindings",
		errorHandler(configurationbinding.Create)),
	"ConfigurationBindingDelete": delete("/namespaces/:namespace/applications/:app/configurationbindings/:configuration",
		errorHandler(configurationbinding.Delete)),

	// List, create, show and delete namespaces
	"Namespaces":           get("/namespaces", errorHandler(namespace.Index)),
	"NamespaceCreate":      post("/namespaces", errorHandler(namespace.Create)),
	"NamespaceDelete":      delete("/namespaces/:namespace", errorHandler(namespace.Delete)),
	"NamespaceBatchDelete": delete("/namespaces", errorHandler(namespace.Delete)),
	"NamespaceShow":        get("/namespaces/:namespace", errorHandler(namespace.Show)),

	// Note, the second registration catches calls with an empty pattern!
	"NamespacesMatch":  get("/namespacematches/:pattern", errorHandler(namespace.Match)),
	"NamespacesMatch0": get("/namespacematches", errorHandler(namespace.Match)),

	// List, show, create and delete configurations
	"ConfigurationApps": get("/namespaces/:namespace/configurationapps", errorHandler(configuration.ConfigurationApps)),
	//
	"AllConfigurations":        get("/configurations", errorHandler(configuration.FullIndex)),
	"Configurations":           get("/namespaces/:namespace/configurations", errorHandler(configuration.Index)),
	"ConfigurationShow":        get("/namespaces/:namespace/configurations/:configuration", errorHandler(configuration.Show)),
	"ConfigurationCreate":      post("/namespaces/:namespace/configurations", errorHandler(configuration.Create)),
	"ConfigurationBatchDelete": delete("/namespaces/:namespace/configurations", errorHandler(configuration.Delete)),
	"ConfigurationDelete":      delete("/namespaces/:namespace/configurations/:configuration", errorHandler(configuration.Delete)),
	"ConfigurationUpdate":      patch("/namespaces/:namespace/configurations/:configuration", errorHandler(configuration.Update)),
	"ConfigurationReplace":     put("/namespaces/:namespace/configurations/:configuration", errorHandler(configuration.Replace)),

	"ConfigurationMatch":  get("/namespaces/:namespace/configurationsmatches/:pattern", errorHandler(configuration.Match)),
	"ConfigurationMatch0": get("/namespaces/:namespace/configurationsmatches", errorHandler(configuration.Match)),

	// Service Catalog
	"ServiceCatalog":     get("/catalogservices", errorHandler(service.Catalog)),
	"ServiceCatalogShow": get("/catalogservices/:catalogservice", errorHandler(service.CatalogShow)),

	// Note, the second registration catches calls with an empty pattern!
	"ServiceCatalogMatch":  get("catalogservicesmatches/:pattern", errorHandler(service.CatalogMatch)),
	"ServiceCatalogMatch0": get("catalogservicesmatches", errorHandler(service.CatalogMatch)),

	// Services
	"ServiceApps": get("/namespaces/:namespace/serviceapps", errorHandler(service.ServiceApps)),
	//
	"AllServices":        get("/services", errorHandler(service.FullIndex)),
	"ServiceCreate":      post("/namespaces/:namespace/services", errorHandler(service.Create)),
	"ServiceList":        get("/namespaces/:namespace/services", errorHandler(service.List)),
	"ServiceShow":        get("/namespaces/:namespace/services/:service", errorHandler(service.Show)),
	"ServiceDelete":      delete("/namespaces/:namespace/services/:service", errorHandler(service.Delete)),
	"ServiceBatchDelete": delete("/namespaces/:namespace/services", errorHandler(service.Delete)),
	"ServiceUpdate":      patch("/namespaces/:namespace/services/:service", errorHandler(service.Update)),
	"ServiceReplace":     put("/namespaces/:namespace/services/:service", errorHandler(service.Replace)),

	"ServiceMatch":  get("/namespaces/:namespace/servicesmatches/:pattern", errorHandler(service.Match)),
	"ServiceMatch0": get("/namespaces/:namespace/servicesmatches", errorHandler(service.Match)),

	// Bind a service to/from applications
	"ServiceBind": post(
		"/namespaces/:namespace/services/:service/bind",
		errorHandler(service.Bind)),

	// Unbind a service to/from applications
	"ServiceUnbind": post(
		"/namespaces/:namespace/services/:service/unbind",
		errorHandler(service.Unbind)),

	// App charts
	"ChartList":   get("/appcharts", errorHandler(appchart.Index)),
	"ChartMatch":  get("/appchartsmatch/:pattern", errorHandler(appchart.Match)),
	"ChartMatch0": get("/appchartsmatch", errorHandler(appchart.Match)),
	"ChartShow":   get("/appcharts/:name", errorHandler(appchart.Show)),

	// Git configurations (auth for private git repos) - List, create, delete, and show.
	"Gitconfigs":           get("/gitconfigs", errorHandler(gitconfig.Index)),
	"GitconfigCreate":      post("/gitconfigs", errorHandler(gitconfig.Create)),
	"GitconfigDelete":      delete("/gitconfigs/:gitconfig", errorHandler(gitconfig.Delete)),
	"GitconfigBatchDelete": delete("/gitconfigs", errorHandler(gitconfig.Delete)),
	"GitconfigsMatch":      get("/gitconfigsmatch/:pattern", errorHandler(gitconfig.Match)),
	"GitconfigsMatch0":     get("/gitconfigsmatch", errorHandler(gitconfig.Match)),
	"GitconfigShow":        get("/gitconfigs/:gitconfig", errorHandler(gitconfig.Show)),

	// List and match export registries - Note: Operators are responsible for creation and deletion.
	"Exportregistries": get("/exportregistries", errorHandler(exportregistry.Index)),
	// Note, the second registration catches calls with an empty pattern!
	"ExportregistriesMatch":  get("/exportregistrymatches/:pattern", errorHandler(exportregistry.Match)),
	"ExportregistriesMatch0": get("/exportregistrymatches", errorHandler(exportregistry.Match)),

	"GitProxy": post("/gitproxy", errorHandler(gitproxy.ProxyHandler)),
}

var WsRoutes = routes.NamedRoutes{
	"AppExec":            get("/namespaces/:namespace/applications/:app/exec", errorHandler(application.Exec)),
	"AppPortForward":     get("/namespaces/:namespace/applications/:app/portforward", errorHandler(application.PortForward)),
	"AppLogs":            get("/namespaces/:namespace/applications/:app/logs", application.Logs),
	"ServicePortForward": get("/namespaces/:namespace/services/:service/portforward", errorHandler(service.PortForward)),
	"StagingLogs":        get("/namespaces/:namespace/staging/:stage_id/logs", application.Logs),
}

// Lemon extends the specified router with the methods and urls
// handling the API endpoints
func Lemon(router *gin.RouterGroup) {
	for _, r := range Routes {
		router.Handle(r.Method, r.Path, r.Handler)
	}
}

// Spice extends the specified router with the methods and urls
// handling the websocket API endpoints
func Spice(router *gin.RouterGroup) {
	for _, r := range WsRoutes {
		router.Handle(r.Method, r.Path, r.Handler)
	}
}

// InitAuthAndRoles will init the Auth (loading the Actions and Endpoints) and the Roles
func InitAuthAndRoles(rolesGetter auth.RolesGetter) error {
	if err := InitAuth(); err != nil {
		return err
	}

	return auth.InitRoles(rolesGetter)
}

// InitAuth will init the Actions, and it will add to the actions the relevant endpoints.
// This was needed because the Routes cannot be referred in the auth package for a cycle dependency.
func InitAuth() error {
	actions, err := auth.InitActions()
	if err != nil {
		return err
	}

	// maps to check the routes and wsRoutes assigned to the actions.
	// They are used to validate that we have put every API to at least an action
	// and that we have not forgotten to map any of them.
	assignedRoutes := make(map[string]struct{})
	assignedWsRoutes := make(map[string]struct{})

	for _, a := range actions {
		for _, routeID := range a.Routes {
			endpoint := auth.NewEndpoint(Routes[routeID])
			a.Endpoints = append(a.Endpoints, endpoint)

			assignedRoutes[routeID] = struct{}{}
		}

		for _, routeID := range a.WsRoutes {
			endpoint := auth.NewWsEndpoint(WsRoutes[routeID])
			a.Endpoints = append(a.Endpoints, endpoint)

			assignedWsRoutes[routeID] = struct{}{}
		}

		auth.ActionsMap[a.ID] = a
	}

	if err := validateActionRoutes(Routes, assignedRoutes); err != nil {
		return err
	}

	if err := validateActionRoutes(WsRoutes, assignedWsRoutes); err != nil {
		return err
	}

	return nil
}

// validateActionRoutes will check if the routes were mapped to actions.
// This prevents the existence of unavailable routes that cannot be reached.
func validateActionRoutes(availableRoutes routes.NamedRoutes, assignedRoutes map[string]struct{}) error {
	for routeID := range availableRoutes {
		if _, found := assignedRoutes[routeID]; !found {
			return fmt.Errorf("route %s is unreachable. Not assigned to any action", routeID)
		}
	}
	return nil
}
