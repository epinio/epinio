package cli

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	apiv1 "github.com/suse/carrier/internal/api/v1"
	"github.com/suse/carrier/internal/filesystem"
	"github.com/suse/carrier/internal/web"
)

func init() {
	flags := CmdServer.Flags()
	flags.Int("port", 0, "(PORT) The port to listen on. Leave empty to auto-assign a random port")
	viper.BindPFlag("port", flags.Lookup("port"))
	viper.BindEnv("port", "PORT")
}

// CmdServer implements the carrier server command
var CmdServer = &cobra.Command{
	Use:   "server",
	Short: "starts the Carrier server. You can connect to it using either your browser or the Carrier client.",
	RunE: func(cmd *cobra.Command, args []string) error {
		httpServerWg := &sync.WaitGroup{}
		httpServerWg.Add(1)
		_, err := startCarrierServer(httpServerWg, viper.GetInt("port"))
		if err != nil {
			return err
		}
		httpServerWg.Wait()

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func startCarrierServer(wg *sync.WaitGroup, port int) (*http.Server, error) {
	listener, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		return nil, err
	}

	// TODO: Use `ui` package?
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
	srv := &http.Server{Handler: nil}
	go func() {
		defer wg.Done() // let caller know we are done cleaning up

		// always returns error. ErrServerClosed on graceful close
		if err := srv.Serve(listener); err != http.ErrServerClosed {
			log.Fatalf("Carrier server failed to start: %v", err)
		}
	}()

	return srv, nil
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
