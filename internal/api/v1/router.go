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

func get(name, path string, h gin.HandlerFunc) routes.Route {
	return routes.NewRoute(name, "GET", path, h)
}

func post(name, path string, h gin.HandlerFunc) routes.Route {
	return routes.NewRoute(name, "POST", path, h)
}

func delete(name, path string, h gin.HandlerFunc) routes.Route {
	return routes.NewRoute(name, "DELETE", path, h)
}

func patch(name, path string, h gin.HandlerFunc) routes.Route {
	return routes.NewRoute(name, "PATCH", path, h)
}

func put(name, path string, h gin.HandlerFunc) routes.Route {
	return routes.NewRoute(name, "PUT", path, h)
}

// AdminRoutes is the list of restricted routes, only accessible by admins
var AdminRoutes map[string]struct{} = map[string]struct{}{}

var Routes = routes.NamedRoutes{}

var WsRoutes = routes.NamedRoutes{}

func MakeRoutes() []routes.Route {
	routes := []routes.Route{}

	routes = append(routes, get("Info", "/info", errorHandler(Info)))
	routes = append(routes, get("AuthToken", "/authtoken", errorHandler(AuthToken)))

	// app controller files see application/*.go

	appController := application.Controller{}

	routes = append(routes, get("AllApps", "/applications", errorHandler(appController.FullIndex)))
	routes = append(routes, get("Apps", "/namespaces/:namespace/applications", errorHandler(appController.Index)))
	routes = append(routes, post("AppCreate", "/namespaces/:namespace/applications", errorHandler(appController.Create)))
	routes = append(routes, get("AppShow", "/namespaces/:namespace/applications/:app", errorHandler(appController.Show)))
	// See stage.go
	routes = append(routes, get("StagingComplete", "/namespaces/:namespace/staging/:stage_id/complete", errorHandler(appController.Staged)))
	routes = append(routes, delete("AppDelete", "/namespaces/:namespace/applications/:app", errorHandler(appController.Delete)))
	routes = append(routes, delete("AppBatchDelete", "/namespaces/:namespace/applications", errorHandler(appController.Delete)))
	routes = append(routes, get("AppValidateCV", "/namespaces/:namespace/applications/:app/validate-cv", errorHandler(appController.ValidateChartValues)))

	// See upload.go
	routes = append(routes, post("AppUpload", "/namespaces/:namespace/applications/:app/store", errorHandler(appController.Upload)))
	routes = append(routes, post("AppImportGit", "/namespaces/:namespace/applications/:app/import-git", errorHandler(appController.ImportGit)))
	// See stage.go
	routes = append(routes, post("AppStage", "/namespaces/:namespace/applications/:app/stage", errorHandler(appController.Stage)))
	routes = append(routes, post("AppDeploy", "/namespaces/:namespace/applications/:app/deploy", errorHandler(appController.Deploy)))
	routes = append(routes, post("AppRestart", "/namespaces/:namespace/applications/:app/restart", errorHandler(appController.Restart)))
	routes = append(routes, patch("AppUpdate", "/namespaces/:namespace/applications/:app", errorHandler(appController.Update)))
	routes = append(routes, get("AppRunning", "/namespaces/:namespace/applications/:app/running", errorHandler(appController.Running)))
	routes = append(routes, get("AppPart", "/namespaces/:namespace/applications/:app/part/:part", errorHandler(appController.GetPart)))

	routes = append(routes, get("AppMatch", "/namespaces/:namespace/appsmatches/:pattern", errorHandler(appController.Match)))
	routes = append(routes, get("AppMatch0", "/namespaces/:namespace/appsmatches", errorHandler(appController.Match)))

	// environment
	envController := env.Controller{}

	// See env.go
	routes = append(routes, get("EnvList", "/namespaces/:namespace/applications/:app/environment", errorHandler(envController.Index)))

	// Note, the second registration catches calls with an empty pattern!
	routes = append(routes, get("EnvMatch", "/namespaces/:namespace/applications/:app/environmentmatch/:pattern", errorHandler(envController.Match)))
	routes = append(routes, get("EnvMatch0", "/namespaces/:namespace/applications/:app/environmentmatch", errorHandler(envController.Match)))

	routes = append(routes, post("EnvSet", "/namespaces/:namespace/applications/:app/environment", errorHandler(envController.Set)))
	routes = append(routes, get("EnvShow", "/namespaces/:namespace/applications/:app/environment/:env", errorHandler(envController.Show)))
	routes = append(routes, delete("EnvUnset", "/namespaces/:namespace/applications/:app/environment/:env", errorHandler(envController.Unset)))

	// configuration binding
	configBindController := configurationbinding.Controller{}

	// Bind and unbind configurations to/from applications, by means of configurationbindings in applications
	routes = append(routes,
		post(
			"ConfigurationBindingCreate",
			"/namespaces/:namespace/applications/:app/configurationbindings",
			errorHandler(configBindController.Create),
		),
	)
	routes = append(routes,
		delete(
			"ConfigurationBindingDelete",
			"/namespaces/:namespace/applications/:app/configurationbindings/:configuration",
			errorHandler(configBindController.Delete),
		),
	)

	// configuration
	configController := configuration.Controller{}

	// List, show, create and delete configurations
	routes = append(routes, get("ConfigurationApps", "/namespaces/:namespace/configurationapps", errorHandler(configController.ConfigurationApps)))
	//
	routes = append(routes, get("AllConfigurations", "/configurations", errorHandler(configController.FullIndex)))
	routes = append(routes, get("Configurations", "/namespaces/:namespace/configurations", errorHandler(configController.Index)))
	routes = append(routes, get("ConfigurationShow", "/namespaces/:namespace/configurations/:configuration", errorHandler(configController.Show)))
	routes = append(routes, post("ConfigurationCreate", "/namespaces/:namespace/configurations", errorHandler(configController.Create)))
	routes = append(routes, delete("ConfigurationDelete", "/namespaces/:namespace/configurations/:configuration", errorHandler(configController.Delete)))
	routes = append(routes, delete("ConfigurationBatchDelete", "/namespaces/:namespace/configurations", errorHandler(configController.Delete)))

	routes = append(routes, patch("ConfigurationUpdate", "/namespaces/:namespace/configurations/:configuration", errorHandler(configController.Update)))
	routes = append(routes, put("ConfigurationReplace", "/namespaces/:namespace/configurations/:configuration", errorHandler(configController.Replace)))

	routes = append(routes, get("ConfigurationMatch", "/namespaces/:namespace/configurationsmatches/:pattern", errorHandler(configController.Match)))
	routes = append(routes, get("ConfigurationMatch0", "/namespaces/:namespace/configurationsmatches", errorHandler(configController.Match)))

	// Services
	serviceController := service.Controller{}

	routes = append(routes, get("ServiceApps", "/namespaces/:namespace/serviceapps", errorHandler(serviceController.ServiceApps)))
	//
	routes = append(routes, get("AllServices", "/services", errorHandler(serviceController.FullIndex)))
	routes = append(routes, get("ServiceCatalog", "/services", errorHandler(serviceController.Catalog)))
	routes = append(routes, get("ServiceCatalogShow", "/services/:catalogservice", errorHandler(serviceController.CatalogShow)))
	routes = append(routes, post("ServiceCreate", "/namespaces/:namespace/services", errorHandler(serviceController.Create)))
	routes = append(routes, get("ServiceList", "/namespaces/:namespace/services", errorHandler(serviceController.List)))
	routes = append(routes, get("ServiceShow", "/namespaces/:namespace/services/:service", errorHandler(serviceController.Show)))
	routes = append(routes, delete("ServiceDelete", "/namespaces/:namespace/services/:service", errorHandler(serviceController.Delete)))
	routes = append(routes, delete("ServiceBatchDelete", "/namespaces/:namespace/services", errorHandler(serviceController.Delete)))

	routes = append(routes, get("ServiceMatch", "/namespaces/:namespace/servicesmatches/:pattern", errorHandler(serviceController.Match)))
	routes = append(routes, get("ServiceMatch0", "/namespaces/:namespace/servicesmatches", errorHandler(serviceController.Match)))

	// Service Catalog
	routes = append(routes, get("ServiceCatalog", "/catalogservices", errorHandler(serviceController.Catalog)))
	routes = append(routes, get("ServiceCatalogShow", "/catalogservices/:catalogservice", errorHandler(serviceController.CatalogShow)))
	// Note, the second registration catches calls with an empty pattern!
	routes = append(routes, get("ServiceCatalogMatch", "catalogservicesmatches/:pattern", errorHandler(serviceController.CatalogMatch)))
	routes = append(routes, get("ServiceCatalogMatch0", "catalogservicesmatches", errorHandler(serviceController.CatalogMatch)))

	// Bind a service to/from applications
	routes = append(routes, post("ServiceBind", "/namespaces/:namespace/services/:service/bind", errorHandler(serviceController.Bind)))

	// Unbind a service to/from applications
	routes = append(routes, post("ServiceUnbind", "/namespaces/:namespace/services/:service/unbind", errorHandler(serviceController.Unbind)))

	// App charts
	appchartController := appchart.Controller{}

	routes = append(routes, get("ChartList", "/appcharts", errorHandler(appchartController.Index)))
	routes = append(routes, get("ChartMatch", "/appchartsmatch/:pattern", errorHandler(appchartController.Match)))
	routes = append(routes, get("ChartMatch0", "/appchartsmatch", errorHandler(appchartController.Match)))
	routes = append(routes, get("ChartShow", "/appcharts/:name", errorHandler(appchartController.Show)))

	return routes
}

func MakeNamespaceRoutes(controller *namespace.Controller) []routes.Route {
	routes := []routes.Route{}

	if controller == nil {
		// List, create, show and delete namespaces
		routes = append(routes, get("Namespaces", "/namespaces", nil))
		routes = append(routes, post("NamespaceCreate", "/namespaces", nil))
		routes = append(routes, delete("NamespaceDelete", "/namespaces/:namespace", nil))
		routes = append(routes, get("NamespaceShow", "/namespaces/:namespace", nil))

		// Note, the second registration catches calls with an empty pattern!
		routes = append(routes, get("NamespacesMatch", "/namespacematches/:pattern", nil))
		routes = append(routes, get("NamespacesMatch0", "/namespacematches", nil))

	} else {
		// List, create, show and delete namespaces
		routes = append(routes, get("Namespaces", "/namespaces", errorHandler(controller.Index)))
		routes = append(routes, post("NamespaceCreate", "/namespaces", errorHandler(controller.Create)))
		routes = append(routes, delete("NamespaceDelete", "/namespaces/:namespace", errorHandler(controller.Delete)))
		routes = append(routes, get("NamespaceShow", "/namespaces/:namespace", errorHandler(controller.Show)))

		// Note, the second registration catches calls with an empty pattern!
		routes = append(routes, get("NamespacesMatch", "/namespacematches/:pattern", errorHandler(controller.Match)))
		routes = append(routes, get("NamespacesMatch0", "/namespacematches", errorHandler(controller.Match)))
	}

	return routes
}

func MakeWsRoutes() []routes.Route {
	routes := []routes.Route{}

	appController := application.Controller{}

	routes = append(routes, get("AppExec", "/namespaces/:namespace/applications/:app/exec", errorHandler(appController.Exec)))
	routes = append(routes, get("AppPortForward", "/namespaces/:namespace/applications/:app/portforward", errorHandler(appController.PortForward)))
	routes = append(routes, get("AppLogs", "/namespaces/:namespace/applications/:app/logs", appController.Logs))
	routes = append(routes, get("StagingLogs", "/namespaces/:namespace/staging/:stage_id/logs", appController.Logs))

	return routes
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
