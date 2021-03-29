// Package v1 is the implementation of Carrier's API v1
package v1

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func Router() *httprouter.Router {
	router := httprouter.New()
	router.HandlerFunc("GET", "/api/v1/info", InfoController{}.Info)
	router.HandlerFunc("GET", "/api/v1/org/:org/applications", ApplicationsController{}.Index)
	router.HandlerFunc("GET", "/api/v1/org/:org/applications/:app", ApplicationsController{}.Show)
	router.NotFound = http.NotFoundHandler()

	return router
}
