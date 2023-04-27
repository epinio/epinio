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
	"reflect"
	"runtime"

	"github.com/gin-gonic/gin"

	"github.com/epinio/epinio/helpers/routes"
	"github.com/epinio/epinio/internal/api/v1/appchart"
	"github.com/epinio/epinio/internal/api/v1/application"
	"github.com/epinio/epinio/internal/api/v1/configuration"
	"github.com/epinio/epinio/internal/api/v1/configurationbinding"
	"github.com/epinio/epinio/internal/api/v1/env"
	"github.com/epinio/epinio/internal/api/v1/namespace"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/api/v1/service"
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
	"Info":      get("/info", errorHandler(Info)),
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

	"AppMatch":  get("/namespaces/:namespace/appsmatches/:pattern", errorHandler(application.Match)),
	"AppMatch0": get("/namespaces/:namespace/appsmatches", errorHandler(application.Match)),

	// See env.go
	"EnvList": get("/namespaces/:namespace/applications/:app/environment", errorHandler(env.Controller{}.Index)),

	// Note, the second registration catches calls with an empty pattern!
	"EnvMatch":  get("/namespaces/:namespace/applications/:app/environmentmatch/:pattern", errorHandler(env.Controller{}.Match)),
	"EnvMatch0": get("/namespaces/:namespace/applications/:app/environmentmatch", errorHandler(env.Controller{}.Match)),

	"EnvSet":   post("/namespaces/:namespace/applications/:app/environment", errorHandler(env.Controller{}.Set)),
	"EnvShow":  get("/namespaces/:namespace/applications/:app/environment/:env", errorHandler(env.Controller{}.Show)),
	"EnvUnset": delete("/namespaces/:namespace/applications/:app/environment/:env", errorHandler(env.Controller{}.Unset)),

	// Bind and unbind configurations to/from applications, by means of configurationbindings in applications
	"ConfigurationBindingCreate": post("/namespaces/:namespace/applications/:app/configurationbindings",
		errorHandler(configurationbinding.Controller{}.Create)),
	"ConfigurationBindingDelete": delete("/namespaces/:namespace/applications/:app/configurationbindings/:configuration",
		errorHandler(configurationbinding.Controller{}.Delete)),

	// List, create, show and delete namespaces
	"Namespaces":           get("/namespaces", errorHandler(namespace.Controller{}.Index)),
	"NamespaceCreate":      post("/namespaces", errorHandler(namespace.Controller{}.Create)),
	"NamespaceDelete":      delete("/namespaces/:namespace", errorHandler(namespace.Controller{}.Delete)),
	"NamespaceBatchDelete": delete("/namespaces", errorHandler(namespace.Controller{}.Delete)),
	"NamespaceShow":        get("/namespaces/:namespace", errorHandler(namespace.Controller{}.Show)),

	// Note, the second registration catches calls with an empty pattern!
	"NamespacesMatch":  get("/namespacematches/:pattern", errorHandler(namespace.Controller{}.Match)),
	"NamespacesMatch0": get("/namespacematches", errorHandler(namespace.Controller{}.Match)),

	// List, show, create and delete configurations
	"ConfigurationApps": get("/namespaces/:namespace/configurationapps", errorHandler(configuration.Controller{}.ConfigurationApps)),
	//
	"AllConfigurations":        get("/configurations", errorHandler(configuration.Controller{}.FullIndex)),
	"Configurations":           get("/namespaces/:namespace/configurations", errorHandler(configuration.Controller{}.Index)),
	"ConfigurationShow":        get("/namespaces/:namespace/configurations/:configuration", errorHandler(configuration.Controller{}.Show)),
	"ConfigurationCreate":      post("/namespaces/:namespace/configurations", errorHandler(configuration.Controller{}.Create)),
	"ConfigurationBatchDelete": delete("/namespaces/:namespace/configurations", errorHandler(configuration.Controller{}.Delete)),
	"ConfigurationDelete":      delete("/namespaces/:namespace/configurations/:configuration", errorHandler(configuration.Controller{}.Delete)),
	"ConfigurationUpdate":      patch("/namespaces/:namespace/configurations/:configuration", errorHandler(configuration.Controller{}.Update)),
	"ConfigurationReplace":     put("/namespaces/:namespace/configurations/:configuration", errorHandler(configuration.Controller{}.Replace)),

	"ConfigurationMatch":  get("/namespaces/:namespace/configurationsmatches/:pattern", errorHandler(configuration.Controller{}.Match)),
	"ConfigurationMatch0": get("/namespaces/:namespace/configurationsmatches", errorHandler(configuration.Controller{}.Match)),

	// Service Catalog
	"ServiceCatalog":     get("/catalogservices", errorHandler(service.Controller{}.Catalog)),
	"ServiceCatalogShow": get("/catalogservices/:catalogservice", errorHandler(service.Controller{}.CatalogShow)),

	// Note, the second registration catches calls with an empty pattern!
	"ServiceCatalogMatch":  get("catalogservicesmatches/:pattern", errorHandler(service.Controller{}.CatalogMatch)),
	"ServiceCatalogMatch0": get("catalogservicesmatches", errorHandler(service.Controller{}.CatalogMatch)),

	// Services
	"ServiceApps": get("/namespaces/:namespace/serviceapps", errorHandler(service.Controller{}.ServiceApps)),
	//
	"AllServices":        get("/services", errorHandler(service.Controller{}.FullIndex)),
	"ServiceCreate":      post("/namespaces/:namespace/services", errorHandler(service.Controller{}.Create)),
	"ServiceList":        get("/namespaces/:namespace/services", errorHandler(service.Controller{}.List)),
	"ServiceShow":        get("/namespaces/:namespace/services/:service", errorHandler(service.Controller{}.Show)),
	"ServiceDelete":      delete("/namespaces/:namespace/services/:service", errorHandler(service.Controller{}.Delete)),
	"ServiceBatchDelete": delete("/namespaces/:namespace/services", errorHandler(service.Controller{}.Delete)),

	"ServiceMatch":  get("/namespaces/:namespace/servicesmatches/:pattern", errorHandler(service.Controller{}.Match)),
	"ServiceMatch0": get("/namespaces/:namespace/servicesmatches", errorHandler(service.Controller{}.Match)),

	// Bind a service to/from applications
	"ServiceBind": post(
		"/namespaces/:namespace/services/:service/bind",
		errorHandler(service.Controller{}.Bind)),

	// Unbind a service to/from applications
	"ServiceUnbind": post(
		"/namespaces/:namespace/services/:service/unbind",
		errorHandler(service.Controller{}.Unbind)),

	// App charts
	"ChartList":   get("/appcharts", errorHandler(appchart.Index)),
	"ChartMatch":  get("/appchartsmatch/:pattern", errorHandler(appchart.Match)),
	"ChartMatch0": get("/appchartsmatch", errorHandler(appchart.Match)),
	"ChartShow":   get("/appcharts/:name", errorHandler(appchart.Show)),
}

var WsRoutes = routes.NamedRoutes{
	"AppExec":        get("/namespaces/:namespace/applications/:app/exec", errorHandler(application.Exec)),
	"AppPortForward": get("/namespaces/:namespace/applications/:app/portforward", errorHandler(application.PortForward)),
	"AppLogs":        get("/namespaces/:namespace/applications/:app/logs", application.Logs),
	"StagingLogs":    get("/namespaces/:namespace/staging/:stage_id/logs", application.Logs),
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
