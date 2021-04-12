// Package web implements the Epinio dashboard
package web

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func Router() *httprouter.Router {
	router := httprouter.New()
	router.HandlerFunc("GET", "/", ApplicationsController{}.Index)
	router.HandlerFunc("GET", "/info", InfoController{}.Index)
	router.NotFound = http.NotFoundHandler()

	return router
}
