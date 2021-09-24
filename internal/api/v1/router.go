// Package v1 is the implementation of Epinio's API v1
// It has the router and controllers (handler funcs) for the API server.
package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/epinio/epinio/helpers/routes"
	"github.com/julienschmidt/httprouter"
)

const v = "/api/v1"

// jsonResponse writes the response struct as JSON to the writer
func jsonResponse(w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = w.Write(js)
	return err
}

func jsonErrorResponse(w http.ResponseWriter, responseErrors APIErrors) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	response := ErrorResponse{Errors: responseErrors.Errors()}
	js, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, marshalErr.Error())
		return
	}

	w.WriteHeader(responseErrors.FirstStatus())
	fmt.Fprintln(w, string(js))
}

func errorHandler(action APIActionFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if errors := action(w, r); errors != nil {
			jsonErrorResponse(w, errors)
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
	"Info": get("/info", errorHandler(InfoController{}.Info)),

	"AllApps":         get("/applications", errorHandler(ApplicationsController{}.FullIndex)),
	"Apps":            get("/namespaces/:org/applications", errorHandler(ApplicationsController{}.Index)),
	"AppCreate":       post("/namespaces/:org/applications", errorHandler(ApplicationsController{}.Create)),
	"AppShow":         get("/namespaces/:org/applications/:app", errorHandler(ApplicationsController{}.Show)),
	"AppLogs":         get("/namespaces/:org/applications/:app/logs", ApplicationsController{}.Logs),
	"StagingLogs":     get("/namespaces/:org/staging/:stage_id/logs", ApplicationsController{}.Logs),
	"StagingComplete": get("/namespaces/:org/staging/:stage_id/complete", errorHandler(ApplicationsController{}.Staged)), // See stage.go
	"AppDelete":       delete("/namespaces/:org/applications/:app", errorHandler(ApplicationsController{}.Delete)),
	"AppUpload":       post("/namespaces/:org/applications/:app/store", errorHandler(ApplicationsController{}.Upload)), // See upload.go
	"AppImportGit":    post("/namespaces/:org/applications/:app/import-git", errorHandler(ApplicationsController{}.ImportGit)),
	"AppStage":        post("/namespaces/:org/applications/:app/stage", errorHandler(ApplicationsController{}.Stage)), // See stage.go
	"AppDeploy":       post("/namespaces/:org/applications/:app/deploy", errorHandler(ApplicationsController{}.Deploy)),
	"AppUpdate":       patch("/namespaces/:org/applications/:app", errorHandler(ApplicationsController{}.Update)),
	"AppRunning":      get("/namespaces/:org/applications/:app/running", errorHandler(ApplicationsController{}.Running)),

	// See env.go
	"EnvList": get("/namespaces/:org/applications/:app/environment", errorHandler(ApplicationsController{}.EnvIndex)),

	// Note, the second registration catches calls with an empty pattern!
	"EnvMatch":  get("/namespaces/:org/applications/:app/environment/:env/match/:pattern", errorHandler(ApplicationsController{}.EnvMatch)),
	"EnvMatch0": get("/namespaces/:org/applications/:app/environment/:env/match", errorHandler(ApplicationsController{}.EnvMatch)),

	"EnvSet":   post("/namespaces/:org/applications/:app/environment", errorHandler(ApplicationsController{}.EnvSet)),
	"EnvShow":  get("/namespaces/:org/applications/:app/environment/:env", errorHandler(ApplicationsController{}.EnvShow)),
	"EnvUnset": delete("/namespaces/:org/applications/:app/environment/:env", errorHandler(ApplicationsController{}.EnvUnset)),

	// Bind and unbind services to/from applications, by means of servicebindings in applications
	"ServiceBindingCreate": post("/namespaces/:org/applications/:app/servicebindings",
		errorHandler(ServicebindingsController{}.Create)),
	"ServiceBindingDelete": delete("/namespaces/:org/applications/:app/servicebindings/:service",
		errorHandler(ServicebindingsController{}.Delete)),

	// List, create, show and delete namespaces
	"Namespaces":      get("/namespaces", errorHandler(NamespacesController{}.Index)),
	"NamespaceCreate": post("/namespaces", errorHandler(NamespacesController{}.Create)),
	"NamespaceDelete": delete("/namespaces/:org", errorHandler(NamespacesController{}.Delete)),

	// Note, the second registration catches calls with an empty pattern!
	"NamespacesMatch":  get("/namespacematches/:pattern", errorHandler(NamespacesController{}.Match)),
	"NamespacesMatch0": get("/namespacematches", errorHandler(NamespacesController{}.Match)),

	// List, show, create and delete services, catalog and custom
	"ServiceApps": get("/namespaces/:org/serviceapps", errorHandler(ApplicationsController{}.ServiceApps)),
	//
	"Services":            get("/namespaces/:org/services", errorHandler(ServicesController{}.Index)),
	"ServiceShow":         get("/namespaces/:org/services/:service", errorHandler(ServicesController{}.Show)),
	"ServiceCreateCustom": post("/namespaces/:org/custom-services", errorHandler(ServicesController{}.CreateCustom)),
	"ServiceDelete":       delete("/namespaces/:org/services/:service", errorHandler(ServicesController{}.Delete)),
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
