// Package v1 is the implementation of Epinio's API v1
package v1

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func Router() *httprouter.Router {
	router := httprouter.New()
	router.HandlerFunc("GET", "/api/v1/info", InfoController{}.Info)
	router.HandlerFunc("GET", "/api/v1/orgs/:org/applications", ApplicationsController{}.Index)
	router.HandlerFunc("GET", "/api/v1/orgs/:org/applications/:app", ApplicationsController{}.Show)
	router.HandlerFunc("DELETE", "/api/v1/orgs/:org/applications/:app", ApplicationsController{}.Delete)

	router.HandlerFunc("GET", "/api/v1/orgs", OrganizationsController{}.Index)
	router.HandlerFunc("POST", "/api/v1/orgs", OrganizationsController{}.Create)

	router.NotFound = http.NotFoundHandler()

	return router
}
