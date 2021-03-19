// Package api is the implementation of the Carrier HTTP API
package api

import (
	"fmt"
	"net"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/suse/carrier/internal/filesystem"
)

var localFilesystem bool

func StartServer(listener net.Listener) error {
	// TODO: Use `ui` package
	fmt.Println("listening on", listener.Addr().String())

	http.Handle("/", setupRouter())
	// Static files
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(filesystem.Dir("/assets", localFilesystem))))

	return http.Serve(listener, nil)
}

func setupRouter() *httprouter.Router {
	router := httprouter.New()
	router.HandlerFunc("GET", "/", HomeController{}.Index)
	router.NotFound = http.NotFoundHandler()

	return router
}
