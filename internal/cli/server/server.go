// Package server provides the Epinio http server
package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/filesystem"
	"github.com/epinio/epinio/internal/web"
	"github.com/epinio/epinio/pkg/api/core/v1/errors"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
)

// startEpinioServer is a helper which initializes and start the API server
func Start(wg *sync.WaitGroup, port int, _ *termui.UI, logger logr.Logger) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		return nil, "", err
	}

	elements := strings.Split(listener.Addr().String(), ":")
	listeningPort := elements[len(elements)-1]

	http.Handle("/api/v1/", loggingHandler(apiv1.Router(), logger))
	http.Handle("/ready", readyRouter())
	http.Handle("/", loggingHandler(web.Router(), logger))
	// Static files
	var assetsDir http.FileSystem
	if os.Getenv("LOCAL_FILESYSTEM") == "true" {
		assetsDir = http.Dir(path.Join(".", "assets", "embedded-web-files", "assets"))
	} else {
		assetsDir = filesystem.Assets()
	}
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(assetsDir)))
	srv := &http.Server{Handler: nil}
	go func() {
		defer wg.Done() // let caller know we are done cleaning up

		// always returns error. ErrServerClosed on graceful close
		if err := srv.Serve(listener); err != http.ErrServerClosed {
			log.Fatalf("Epinio server failed to start: %v", err)
		}
	}()

	return srv, listeningPort, nil
}

// readyRouter constructs and returns the router for the endpoint
// handling the kube probes (liveness, readiness)
func readyRouter() *httprouter.Router {
	router := httprouter.New()
	router.HandlerFunc("GET", "/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	return router
}

// loggingHandler is the logging middleware for requests
func loggingHandler(h http.Handler, logger logr.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf("%d", rand.Intn(10000)) // nolint:gosec // Non-crypto use
		user := r.Header.Get("X-Webauth-User")

		log := logger.WithName(id).WithValues(
			"method", r.Method,
			"uri", r.URL.String(),
			"user", user,
		)

		// add our logger
		ctx := r.Context()
		ctx = tracelog.WithLogger(ctx, log)
		ctx = requestctx.ContextWithUser(ctx, user)
		ctx = requestctx.ContextWithID(ctx, id)
		r = r.WithContext(ctx)

		// log the request first, then ...
		logRequest(r, log)

		// ... call the original http.Handler
		if len(user) > 0 {
			h.ServeHTTP(w, r)
		} else {
			log.Error(errors.UserNotFound(), "username not found in the header")
		}

		if log.V(15).Enabled() {
			log = log.WithValues("header", w.Header())
		}
		log.V(5).Info("response written")
	})
}

// logRequest is the logging backend for requests
func logRequest(r *http.Request, log logr.Logger) {
	if log.V(15).Enabled() {
		log = log.WithValues(
			"header", r.Header,
			"params", r.Form,
		)
	}

	// Read request body for logging
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err, "request failed", "body", "error")
		return
	}
	r.Body.Close()

	// Recreate body for the actual handler
	r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	// log body only at higher trace levels
	b := "n/a"
	if len(bodyBytes) != 0 {
		b = string(bodyBytes)
	}
	if log.V(15).Enabled() {
		log = log.WithValues("body", b)
	}

	log.V(1).Info("request received", "bodylen", len(bodyBytes))
}
