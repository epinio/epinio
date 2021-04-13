// Package v1 is the implementation of Epinio's API v1
package v1

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func Router() *httprouter.Router {
	router := httprouter.New()
	router.HandlerFunc("GET", "/api/v1/info", InfoController{}.Info)

	// List, show, and delete applications
	router.HandlerFunc("GET", "/api/v1/orgs/:org/applications", ApplicationsController{}.Index)
	router.HandlerFunc("GET", "/api/v1/orgs/:org/applications/:app", ApplicationsController{}.Show)
	router.HandlerFunc("DELETE", "/api/v1/orgs/:org/applications/:app", ApplicationsController{}.Delete)

	// Bind and unbind services to/from applications
	router.HandlerFunc("POST", "/api/v1/orgs/:org/applications/:app/services", ApplicationsController{}.Bind)
	router.HandlerFunc("DELETE", "/api/v1/orgs/:org/applications/:app/services/:service", ApplicationsController{}.Unbind)

	// List and create organizations
	router.HandlerFunc("GET", "/api/v1/orgs", OrganizationsController{}.Index)
	router.HandlerFunc("POST", "/api/v1/orgs", OrganizationsController{}.Create)

	// List, show, create and delete services, catalog and custom
	router.HandlerFunc("GET", "/api/v1/orgs/:org/services", ServicesController{}.Index)
	router.HandlerFunc("GET", "/api/v1/orgs/:org/services/:service", ServicesController{}.Show)
	router.HandlerFunc("POST", "/api/v1/orgs/:org/services", ServicesController{}.Create)
	router.HandlerFunc("POST", "/api/v1/orgs/:org/custom-services", ServicesController{}.CreateCustom)
	router.HandlerFunc("DELETE", "/api/v1/orgs/:org/services/:service", ServicesController{}.Delete)

	// list service classes and plans (of catalog services)
	router.HandlerFunc("GET", "/api/v1/serviceclasses", ServiceClassesController{}.Index)
	router.HandlerFunc("GET", "/api/v1/serviceclasses/:serviceclass/serviceplans", ServicePlansController{}.Index)

	router.NotFound = http.NotFoundHandler()

	return router
}
