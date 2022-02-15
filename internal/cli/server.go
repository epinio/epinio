package cli

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/server"
	"github.com/epinio/epinio/internal/version"
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

	flags.String("tls-issuer", "", "(TLS_ISSUER) The cluster issuer to use for workload certificates")
	viper.BindPFlag("tls-issuer", flags.Lookup("tls-issuer"))
	viper.BindEnv("tls-issuer", "TLS_ISSUER")

	flags.String("access-control-allow-origin", "", "(ACCESS_CONTROL_ALLOW_ORIGIN) Domains allowed to use the API")
	viper.BindPFlag("access-control-allow-origin", flags.Lookup("access-control-allow-origin"))
	viper.BindEnv("access-control-allow-origin", "ACCESS_CONTROL_ALLOW_ORIGIN")

	flags.String("registry-certificate-secret", "", "(REGISTRY_CERTIFICATE_SECRET) Secret for the registry's TLS certificate")
	viper.BindPFlag("registry-certificate-secret", flags.Lookup("registry-certificate-secret"))
	viper.BindEnv("registry-certificate-secret", "REGISTRY_CERTIFICATE_SECRET")

	flags.String("output", "text", "(OUTPUT) logs output format [text,json]")
	viper.BindPFlag("output", flags.Lookup("output"))
	viper.BindEnv("output", "OUTPUT")

	flags.String("ingress-class-name", "", "(INGRESS_CLASS_NAME) Name of the ingress class to use for apps. Leave empty to add no ingressClassName to the ingress.")
	viper.BindPFlag("ingress-class-name", flags.Lookup("ingress-class-name"))
	viper.BindEnv("ingress-class-name", "INGRESS_CLASS_NAME")
}

// CmdServer implements the command: epinio server
var CmdServer = &cobra.Command{
	Use:   "server",
	Short: "Starts the Epinio server.",
	Long:  "This command starts the Epinio server. `epinio install` ensures the server is running inside your cluster. Normally you don't need to run this command manually.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// init logger
		var logger logr.Logger
		if viper.GetString("output") == "json" {
			logger = tracelog.NewZapLogger()
		} else {
			logger = tracelog.NewLogger()
		}
		logger = logger.WithName("EpinioServer")

		handler, err := server.NewHandler(logger)
		if err != nil {
			return errors.Wrap(err, "error creating handler")
		}

		port := viper.GetInt("port")
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return errors.Wrap(err, "error creating listener")
		}

		ui := termui.NewUI()
		ui.Normal().Msg("Epinio version: " + version.Version)
		listeningPort := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
		ui.Normal().Msg("listening on localhost on port " + listeningPort)

		return startServerGracefully(listener, handler)
	},
}

// startServerGracefully will start the server and will wait for a graceful shutdown
func startServerGracefully(listener net.Listener, handler http.Handler) error {
	srv := &http.Server{
		Handler: handler,
	}

	go func() {
		if err := srv.Serve(listener); err != nil && errors.Is(err, http.ErrServerClosed) {
			log.Printf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
		return err
	}

	log.Println("Server exiting")
	return nil
}
