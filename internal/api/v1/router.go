// Package v1 is the implementation of Epinio's API v1
package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/epinio/epinio/helpers/routes"
	"github.com/julienschmidt/httprouter"
)

const v = "/api/v1"

func errorHandler(action APIActionFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseErrors := action(w, r)
		if len(responseErrors) > 0 {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")

			response := map[string][]APIError{"errors": responseErrors}

			js, marshalErr := json.Marshal(response)
			if marshalErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, marshalErr.Error())
				return
			}

			w.WriteHeader(responseErrors[0].Status)
			fmt.Fprintln(w, string(js))
			return
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

var Routes = routes.NamedRoutes{
	"Info":      get("/info", errorHandler(InfoController{}.Info)),
	"Apps":      get("/orgs/:org/applications", errorHandler(ApplicationsController{}.Index)),
	"AppShow":   get("/orgs/:org/applications/:app", errorHandler(ApplicationsController{}.Show)),
	"AppDelete": delete("/orgs/:org/applications/:app", errorHandler(ApplicationsController{}.Delete)),
	"AppUpload": post("/orgs/:org/applications/:app", errorHandler(ApplicationsController{}.Upload)),

	// Bind and unbind services to/from applications, by means of servicebindings in applications
	"ServiceBindingCreate": post("/orgs/:org/applications/:app/servicebindings",
		errorHandler(ServicebindingsController{}.Create)),
	"ServiceBindingDelete": delete("/orgs/:org/applications/:app/servicebindings/:service",
		errorHandler(ServicebindingsController{}.Delete)),

	// List, create, show and delete organizations
	"Orgs":      get("/orgs", errorHandler(OrganizationsController{}.Index)),
	"OrgCreate": post("/orgs", errorHandler(OrganizationsController{}.Create)),
	"OrgDelete": delete("/orgs/:org", errorHandler(OrganizationsController{}.Delete)),

	// List, show, create and delete services, catalog and custom
	"Services":            get("/orgs/:org/services", errorHandler(ServicesController{}.Index)),
	"ServiceShow":         get("/orgs/:org/services/:service", errorHandler(ServicesController{}.Show)),
	"ServiceCreate":       post("/orgs/:org/services", errorHandler(ServicesController{}.Create)),
	"ServiceCreateCustom": post("/orgs/:org/custom-services", errorHandler(ServicesController{}.CreateCustom)),
	"ServiceDelete":       delete("/orgs/:org/services/:service", errorHandler(ServicesController{}.Delete)),

	// list service classes and plans (of catalog services)
	"ServiceClasses": get("/serviceclasses", errorHandler(ServiceClassesController{}.Index)),
	"ServicePlans":   get("/serviceclasses/:serviceclass/serviceplans", errorHandler(ServicePlansController{}.Index)),
}

func Router() *httprouter.Router {
	router := httprouter.New()

	for _, r := range Routes {
		router.HandlerFunc(r.Method, r.Path, r.Handler)
	}

	router.NotFound = http.NotFoundHandler()

	return router
}
