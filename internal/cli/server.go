package cli

import (
	"bytes"
	"context"
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

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/filesystem"
	"github.com/epinio/epinio/internal/web"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	flags := CmdServer.Flags()
	flags.Int("port", 0, "(PORT) The port to listen on. Leave empty to auto-assign a random port")
	viper.BindPFlag("port", flags.Lookup("port"))
	viper.BindEnv("port", "PORT")

	flags.String("tls-issuer", deployments.EpinioCAIssuer, "(TLS_ISSUER) The cluster issuer to use for workload certificates")
	viper.BindPFlag("tls-issuer", flags.Lookup("tls-issuer"))
	viper.BindEnv("tls-issuer", "TLS_ISSUER")

	flags.Bool("use-internal-registry-node-port", true, "(USE_INTERNAL_REGISTRY_NODE_PORT) Use the internal registry via a node port")
	viper.BindPFlag("use-internal-registry-node-port", flags.Lookup("use-internal-registry-node-port"))
	viper.BindEnv("use-internal-registry-node-port", "USE_INTERNAL_REGISTRY_NODE_PORT")
}

// CmdServer implements the epinio server command
var CmdServer = &cobra.Command{
	Use:   "server",
	Short: "Starts the Epinio server.",
	Long:  "This command starts the Epinio server. `epinio install` ensures the server is running inside your cluster. Normally you don't need to run this command manually.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		httpServerWg := &sync.WaitGroup{}
		httpServerWg.Add(1)
		port := viper.GetInt("port")
		ui := termui.NewUI()
		logger := tracelog.NewServerLogger()
		_, listeningPort, err := startEpinioServer(httpServerWg, port, ui, logger)
		if err != nil {
			return errors.Wrap(err, "failed to start server")
		}
		ui.Normal().Msg("listening on localhost on port " + listeningPort)
		httpServerWg.Wait()

		return nil
	},
}

func startEpinioServer(wg *sync.WaitGroup, port int, ui *termui.UI, logger logr.Logger) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		return nil, "", err
	}

	elements := strings.Split(listener.Addr().String(), ":")
	listeningPort := elements[len(elements)-1]

	http.Handle("/api/v1/", logRequestHandler(apiv1.Router(), logger))
	http.Handle("/", logRequestHandler(web.Router(), logger))
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

// logging middleware for requests
func logRequestHandler(h http.Handler, logger logr.Logger) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf("%d", rand.Intn(10000))
		log := logger.WithName(id)

		// add our logger
		ctx := r.Context()
		ctx = context.WithValue(ctx, tracelog.CtxLoggerKey{}, log)
		r = r.WithContext(ctx)

		// log the request first, then ...
		logRequest(r, log)

		// ... call the original http.Handler
		h.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func logRequest(r *http.Request, log logr.Logger) {
	uri := r.URL.String()
	method := r.Method
	log.V(1).Info("received request", "method", method, "uri", uri)

	// Read request body for logging
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err, "request", "body", "error")
		return
	}
	r.Body.Close()

	// Recreate body for the actual handler
	r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	if len(bodyBytes) == 0 {
		log.V(2).Info("request", "body", "n/a")
		return
	}

	log.V(2).Info("request", "body", string(bodyBytes))
}
