// Package v1 is the implementation of Epinio's API v1
// It has the router and controllers (handler funcs) for the API server.
package v1

import (
	"net/http"
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
	"github.com/julienschmidt/httprouter"
)

const v = "/api/v1"

// APIActionFunc is matched by all actions. Actions can return a list of errors.
// The "Status" of the first error in the list becomes the response Status Code.
type APIActionFunc func(http.ResponseWriter, *http.Request) errors.APIErrors

func funcName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func errorHandler(action APIActionFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if errors := action(w, r); errors != nil {
			tracelog.Logger(r.Context()).V(1).
				Info("responding with json error response", "action", funcName(action), "errors", errors)
			response.JSONError(w, errors)
		}
	}
}

func get(path string, h http.HandlerFunc) routes.Route {
	return routes.NewRoute("GET", v+path, h)
}

func post(path string, h http.HandlerFunc) routes.Route {
	return routes.NewRoute("POST", v+path, h)
}

func delete(path string, h http.HandlerFunc) routes.Route {
	return routes.NewRoute("DELETE", v+path, h)
}

func patch(path string, h http.HandlerFunc) routes.Route {
	return routes.NewRoute("PATCH", v+path, h)
}

var Routes = routes.NamedRoutes{
	"Info": get("/info", errorHandler(Info)),

	"AllApps":         get("/applications", errorHandler(application.Controller{}.FullIndex)),
	"Apps":            get("/namespaces/:org/applications", errorHandler(application.Controller{}.Index)),
	"AppCreate":       post("/namespaces/:org/applications", errorHandler(application.Controller{}.Create)),
	"AppShow":         get("/namespaces/:org/applications/:app", errorHandler(application.Controller{}.Show)),
	"AppLogs":         get("/namespaces/:org/applications/:app/logs", application.Controller{}.Logs),
	"StagingLogs":     get("/namespaces/:org/staging/:stage_id/logs", application.Controller{}.Logs),
	"StagingComplete": get("/namespaces/:org/staging/:stage_id/complete", errorHandler(application.Controller{}.Staged)), // See stage.go
	"AppDelete":       delete("/namespaces/:org/applications/:app", errorHandler(application.Controller{}.Delete)),
	"AppUpload":       post("/namespaces/:org/applications/:app/store", errorHandler(application.Controller{}.Upload)), // See upload.go
	"AppImportGit":    post("/namespaces/:org/applications/:app/import-git", errorHandler(application.Controller{}.ImportGit)),
	"AppStage":        post("/namespaces/:org/applications/:app/stage", errorHandler(application.Controller{}.Stage)), // See stage.go
	"AppDeploy":       post("/namespaces/:org/applications/:app/deploy", errorHandler(application.Controller{}.Deploy)),
	"AppUpdate":       patch("/namespaces/:org/applications/:app", errorHandler(application.Controller{}.Update)),
	"AppRunning":      get("/namespaces/:org/applications/:app/running", errorHandler(application.Controller{}.Running)),

	// See env.go
	"EnvList": get("/namespaces/:org/applications/:app/environment", errorHandler(env.Controller{}.Index)),

	// Note, the second registration catches calls with an empty pattern!
	"EnvMatch":  get("/namespaces/:org/applications/:app/environment/:env/match/:pattern", errorHandler(env.Controller{}.Match)),
	"EnvMatch0": get("/namespaces/:org/applications/:app/environment/:env/match", errorHandler(env.Controller{}.Match)),

	"EnvSet":   post("/namespaces/:org/applications/:app/environment", errorHandler(env.Controller{}.Set)),
	"EnvShow":  get("/namespaces/:org/applications/:app/environment/:env", errorHandler(env.Controller{}.Show)),
	"EnvUnset": delete("/namespaces/:org/applications/:app/environment/:env", errorHandler(env.Controller{}.Unset)),

	// Bind and unbind services to/from applications, by means of servicebindings in applications
	"ServiceBindingCreate": post("/namespaces/:org/applications/:app/servicebindings",
		errorHandler(servicebinding.Controller{}.Create)),
	"ServiceBindingDelete": delete("/namespaces/:org/applications/:app/servicebindings/:service",
		errorHandler(servicebinding.Controller{}.Delete)),

	// List, create, show and delete namespaces
	"Namespaces":      get("/namespaces", errorHandler(namespace.Controller{}.Index)),
	"NamespaceCreate": post("/namespaces", errorHandler(namespace.Controller{}.Create)),
	"NamespaceDelete": delete("/namespaces/:org", errorHandler(namespace.Controller{}.Delete)),

	// Note, the second registration catches calls with an empty pattern!
	"NamespacesMatch":  get("/namespacematches/:pattern", errorHandler(namespace.Controller{}.Match)),
	"NamespacesMatch0": get("/namespacematches", errorHandler(namespace.Controller{}.Match)),

	// List, show, create and delete services
	"ServiceApps": get("/namespaces/:org/serviceapps", errorHandler(service.Controller{}.ServiceApps)),
	//
	"Services":      get("/namespaces/:org/services", errorHandler(service.Controller{}.Index)),
	"ServiceShow":   get("/namespaces/:org/services/:service", errorHandler(service.Controller{}.Show)),
	"ServiceCreate": post("/namespaces/:org/services", errorHandler(service.Controller{}.Create)),
	"ServiceDelete": delete("/namespaces/:org/services/:service", errorHandler(service.Controller{}.Delete)),
}

// Router constructs and returns the router mapping methods and urls to the API handlers.
func Router() *httprouter.Router {
	router := httprouter.New()

	for _, r := range Routes {
		router.HandlerFunc(r.Method, r.Path, r.Handler)
	}

	router.NotFound = http.NotFoundHandler()

	return router
}
