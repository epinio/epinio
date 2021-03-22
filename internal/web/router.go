// Package web implements the Carrier dashboard
package web

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func Router() *httprouter.Router {
	router := httprouter.New()
	router.HandlerFunc("GET", "/", HomeController{}.Index)
	router.NotFound = http.NotFoundHandler()

	return router
}
