package cli

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/filesystem"
	"github.com/epinio/epinio/internal/web"
	"github.com/epinio/epinio/termui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	flags := CmdServer.Flags()
	flags.Int("port", 0, "(PORT) The port to listen on. Leave empty to auto-assign a random port")
	viper.BindPFlag("port", flags.Lookup("port"))
	viper.BindEnv("port", "PORT")
}

// CmdServer implements the epinio server command
var CmdServer = &cobra.Command{
	Use:   "server",
	Short: "starts the Epinio server. You can connect to it using either your browser or the Epinio client.",
	RunE: func(cmd *cobra.Command, args []string) error {
		httpServerWg := &sync.WaitGroup{}
		httpServerWg.Add(1)
		port := viper.GetInt("port")
		ui := termui.NewUI()
		_, listeningPort, err := startEpinioServer(httpServerWg, port, ui)
		if err != nil {
			return err
		}
		fmt.Println("listening on localhost on port " + listeningPort)
		httpServerWg.Wait()

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func startEpinioServer(wg *sync.WaitGroup, port int, ui *termui.UI) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		return nil, "", err
	}

	elements := strings.Split(listener.Addr().String(), ":")
	listeningPort := elements[len(elements)-1]

	http.Handle("/api/v1/", logRequestHandler(apiv1.Router(), ui))
	http.Handle("/", logRequestHandler(web.Router(), ui))
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
			log.Fatalf("Epinio server failed to start: %v", err)
		}
	}()

	return srv, listeningPort, nil
}

// loggingmiddleware for requests
func logRequestHandler(h http.Handler, ui *termui.UI) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {

		// call the original http.Handler
		h.ServeHTTP(w, r)

		// log the request
		uri := r.URL.String()
		method := r.Method
		ui.Normal().V(1).Msgf("%s %s", method, uri)
	}

	return http.HandlerFunc(fn)
}
