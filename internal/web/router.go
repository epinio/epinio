// Package web implements the Epinio dashboard
package web

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func Router() *httprouter.Router {
	router := httprouter.New()
	router.HandlerFunc("GET", "/", ApplicationsController{}.Index)
	router.HandlerFunc("GET", "/login", DexController{}.Login)
	router.HandlerFunc("GET", "/dex/callback", DexController{}.Callback)
	router.HandlerFunc("GET", "/info", InfoController{}.Index)
	router.HandlerFunc("GET", "/orgs/target/:org", OrgsController{}.Target)
	router.NotFound = http.NotFoundHandler()

	return router
}
