// Package v1 is the implementation of Epinio's API v1
// It has the router and controllers (handler funcs) for the API server.
package v1

import (
	"reflect"
	"runtime"

	"github.com/epinio/epinio/helpers/routes"
	"github.com/epinio/epinio/helpers/tracelog"

	"github.com/epinio/epinio/internal/api/v1/application"
	"github.com/epinio/epinio/internal/api/v1/env"
	"github.com/epinio/epinio/internal/api/v1/namespace"
	"github.com/epinio/epinio/internal/api/v1/response"

	"github.com/epinio/epinio/internal/api/v1/service"
	"github.com/epinio/epinio/internal/api/v1/servicebinding"
	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
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
			tracelog.Logger(c.Request.Context()).V(1).
				Info("responding with json error response", "action", funcName(action), "errors", errors)
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

var Routes = routes.NamedRoutes{
	"Info":      get("/info", errorHandler(Info)),
	"AuthToken": get("/authtoken", errorHandler(AuthToken)),

	// app controller files see application/*.go

	"AllApps":         get("/applications", errorHandler(application.Controller{}.FullIndex)),
	"Apps":            get("/namespaces/:namespace/applications", errorHandler(application.Controller{}.Index)),
	"AppCreate":       post("/namespaces/:namespace/applications", errorHandler(application.Controller{}.Create)),
	"AppShow":         get("/namespaces/:namespace/applications/:app", errorHandler(application.Controller{}.Show)),
	"StagingComplete": get("/namespaces/:namespace/staging/:stage_id/complete", errorHandler(application.Controller{}.Staged)), // See stage.go
	"AppDelete":       delete("/namespaces/:namespace/applications/:app", errorHandler(application.Controller{}.Delete)),
	"AppUpload":       post("/namespaces/:namespace/applications/:app/store", errorHandler(application.Controller{}.Upload)), // See upload.go
	"AppImportGit":    post("/namespaces/:namespace/applications/:app/import-git", errorHandler(application.Controller{}.ImportGit)),
	"AppStage":        post("/namespaces/:namespace/applications/:app/stage", errorHandler(application.Controller{}.Stage)), // See stage.go
	"AppDeploy":       post("/namespaces/:namespace/applications/:app/deploy", errorHandler(application.Controller{}.Deploy)),
	"AppRestart":      post("/namespaces/:namespace/applications/:app/restart", errorHandler(application.Controller{}.Restart)),
	"AppUpdate":       patch("/namespaces/:namespace/applications/:app", errorHandler(application.Controller{}.Update)),
	"AppRunning":      get("/namespaces/:namespace/applications/:app/running", errorHandler(application.Controller{}.Running)),
	"AppExec":         get("/namespaces/:namespace/applications/:app/exec", errorHandler(application.Controller{}.Exec)),
	"AppPortForward":  get("/namespaces/:namespace/applications/:app/portforward", errorHandler(application.Controller{}.PortForward)),

	// See env.go
	"EnvList": get("/namespaces/:namespace/applications/:app/environment", errorHandler(env.Controller{}.Index)),

	// Note, the second registration catches calls with an empty pattern!
	"EnvMatch":  get("/namespaces/:namespace/applications/:app/environmentmatch/:pattern", errorHandler(env.Controller{}.Match)),
	"EnvMatch0": get("/namespaces/:namespace/applications/:app/environmentmatch", errorHandler(env.Controller{}.Match)),

	"EnvSet":   post("/namespaces/:namespace/applications/:app/environment", errorHandler(env.Controller{}.Set)),
	"EnvShow":  get("/namespaces/:namespace/applications/:app/environment/:env", errorHandler(env.Controller{}.Show)),
	"EnvUnset": delete("/namespaces/:namespace/applications/:app/environment/:env", errorHandler(env.Controller{}.Unset)),

	// Bind and unbind services to/from applications, by means of servicebindings in applications
	"ServiceBindingCreate": post("/namespaces/:namespace/applications/:app/servicebindings",
		errorHandler(servicebinding.Controller{}.Create)),
	"ServiceBindingDelete": delete("/namespaces/:namespace/applications/:app/servicebindings/:service",
		errorHandler(servicebinding.Controller{}.Delete)),

	// List, create, show and delete namespaces
	"Namespaces":      get("/namespaces", errorHandler(namespace.Controller{}.Index)),
	"NamespaceCreate": post("/namespaces", errorHandler(namespace.Controller{}.Create)),
	"NamespaceDelete": delete("/namespaces/:namespace", errorHandler(namespace.Controller{}.Delete)),
	"NamespaceShow":   get("/namespaces/:namespace", errorHandler(namespace.Controller{}.Show)),

	// Note, the second registration catches calls with an empty pattern!
	"NamespacesMatch":  get("/namespacematches/:pattern", errorHandler(namespace.Controller{}.Match)),
	"NamespacesMatch0": get("/namespacematches", errorHandler(namespace.Controller{}.Match)),

	// List, show, create and delete services
	"ServiceApps": get("/namespaces/:namespace/serviceapps", errorHandler(service.Controller{}.ServiceApps)),
	//
	"AllServices":    get("/services", errorHandler(service.Controller{}.FullIndex)),
	"Services":       get("/namespaces/:namespace/services", errorHandler(service.Controller{}.Index)),
	"ServiceShow":    get("/namespaces/:namespace/services/:service", errorHandler(service.Controller{}.Show)),
	"ServiceCreate":  post("/namespaces/:namespace/services", errorHandler(service.Controller{}.Create)),
	"ServiceDelete":  delete("/namespaces/:namespace/services/:service", errorHandler(service.Controller{}.Delete)),
	"ServiceUpdate":  patch("/namespaces/:namespace/services/:service", errorHandler(service.Controller{}.Update)),
	"ServiceReplace": put("/namespaces/:namespace/services/:service", errorHandler(service.Controller{}.Replace)),
}

var WsRoutes = routes.NamedRoutes{
	"AppExec":     get("/namespaces/:namespace/applications/:app/exec", errorHandler(application.Controller{}.Exec)),
	"AppLogs":     get("/namespaces/:namespace/applications/:app/logs", application.Controller{}.Logs),
	"StagingLogs": get("/namespaces/:namespace/staging/:stage_id/logs", application.Controller{}.Logs),
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
