// Package v1 is the implementation of Epinio's API v1
package v1

import (
	"net/http"

	"github.com/epinio/epinio/helpers/routes"
	"github.com/julienschmidt/httprouter"
)

const v = "/api/v1"

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
	"Info":      get("/info", InfoController{}.Info),
	"Apps":      get("/orgs/:org/applications", ApplicationsController{}.Index),
	"AppShow":   get("/orgs/:org/applications/:app", ApplicationsController{}.Show),
	"AppDelete": delete("/orgs/:org/applications/:app", ApplicationsController{}.Delete),
	"AppUpload": post("/orgs/:org/applications/:app", ApplicationsController{}.Upload),

	// Bind and unbind services to/from applications, by means of servicebindings in applications
	"ServiceBindingCreate": post("/orgs/:org/applications/:app/servicebindings", ServicebindingsController{}.Create),
	"ServiceBindingDelete": delete("/orgs/:org/applications/:app/servicebindings/:service", ServicebindingsController{}.Delete),

	// List and create organizations
	"Orgs":      get("/orgs", OrganizationsController{}.Index),
	"OrgCreate": post("/orgs", OrganizationsController{}.Create),

	// List, show, create and delete services, catalog and custom
	"Services":            get("/orgs/:org/services", ServicesController{}.Index),
	"ServiceShow":         get("/orgs/:org/services/:service", ServicesController{}.Show),
	"ServiceCreate":       post("/orgs/:org/services", ServicesController{}.Create),
	"ServiceCreateCustom": post("/orgs/:org/custom-services", ServicesController{}.CreateCustom),
	"ServiceDelete":       delete("/orgs/:org/services/:service", ServicesController{}.Delete),

	// list service classes and plans (of catalog services)
	"ServiceClasses": get("/serviceclasses", ServiceClassesController{}.Index),
	"ServicePlans":   get("/serviceclasses/:serviceclass/serviceplans", ServicePlansController{}.Index),
}

func Router() *httprouter.Router {
	router := httprouter.New()

	for _, r := range Routes {
		router.HandlerFunc(r.Method, r.Path, r.Handler)
	}

	router.NotFound = http.NotFoundHandler()

	return router
}
