// Package web implements the Epinio dashboard
package web

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Router constructs and returns the router mapping methods and urls to the dashboard handlers.
func Router() *httprouter.Router {
	router := httprouter.New()
	router.HandlerFunc("GET", "/", ApplicationsController{}.Index)
	router.HandlerFunc("GET", "/info", InfoController{}.Index)
	router.HandlerFunc("GET", "/orgs/target/:org", OrgsController{}.Target)
	router.NotFound = http.NotFoundHandler()

	return router
}
