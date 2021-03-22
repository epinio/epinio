// Package api is the implementation of the Carrier HTTP API
package api

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path"

	apiv1 "github.com/suse/carrier/internal/api/v1"
	"github.com/suse/carrier/internal/filesystem"
	"github.com/suse/carrier/internal/web"
)

func StartServer(listener net.Listener) error {
	// TODO: Use `ui` package
	fmt.Println("listening on", listener.Addr().String())

	http.Handle("/api/v1/", logRequestHandler(apiv1.Router()))
	http.Handle("/", logRequestHandler(web.Router()))
	// Static files
	var assetsDir http.FileSystem
	if os.Getenv("LOCAL_FILESYSTEM") == "true" {
		assetsDir = http.Dir(path.Join(".", "embedded-web-files", "assets"))
	} else {
		assetsDir = filesystem.Assets()
	}
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(assetsDir)))

	return http.Serve(listener, nil)
}

// loggingmiddleware for requests
func logRequestHandler(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {

		// call the original http.Handler
		h.ServeHTTP(w, r)

		// log the request
		uri := r.URL.String()
		method := r.Method
		// TODO: Use verbosity level to decide if we print or not
		fmt.Printf("%s %s\n", method, uri)
	}

	return http.HandlerFunc(fn)
}
