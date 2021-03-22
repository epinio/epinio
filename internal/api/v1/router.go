package v1

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func Router() *httprouter.Router {
	router := httprouter.New()
	router.HandlerFunc("GET", "/api/v1/applications", ApplicationsController{}.Index)
	router.NotFound = http.NotFoundHandler()

	return router
}
