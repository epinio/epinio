// Package api is the implementation of the Carrier HTTP API
package api

import (
	"fmt"
	"net"
	"net/http"
	"path"

	"github.com/julienschmidt/httprouter"
	"github.com/suse/carrier/internal/filesystem"
)

var localFilesystem bool

func StartServer(listener net.Listener, userLocalFilesystem bool) error {
	// TODO: Use `ui` package
	fmt.Println("listening on", listener.Addr().String())

	localFilesystem = userLocalFilesystem

	http.Handle("/", setupRouter())
	// Static files
	var assetsDir http.FileSystem
	if localFilesystem {
		assetsDir = http.Dir(path.Join(".", "embedded-web-files", "assets"))
	} else {
		assetsDir = filesystem.Assets()
	}
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(assetsDir)))

	return http.Serve(listener, nil)
}

func setupRouter() *httprouter.Router {
	router := httprouter.New()
	router.HandlerFunc("GET", "/", HomeController{}.Index)
	router.NotFound = http.NotFoundHandler()

	return router
}
