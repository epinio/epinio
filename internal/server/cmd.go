// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package server provides the Epinio server command and HTTP server lifecycle.
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/upgraderesponder"
	"github.com/epinio/epinio/internal/version"
	"github.com/gin-gonic/gin"

	"k8s.io/client-go/rest"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	flags := CmdServer.Flags()

	flags.StringP("namespace", "n", "epinio", "(NAMESPACE) The namespace to use")
	checkErr(viper.BindPFlag("namespace", flags.Lookup("namespace")))
	checkErr(viper.BindEnv("namespace", "NAMESPACE"))

	flags.Int("port", 0, "(PORT) The port to listen on. Leave empty to auto-assign a random port")
	checkErr(viper.BindPFlag("port", flags.Lookup("port")))
	checkErr(viper.BindEnv("port", "PORT"))

	flags.String("tls-issuer", "", "(TLS_ISSUER) The cluster issuer to use for workload certificates")
	checkErr(viper.BindPFlag("tls-issuer", flags.Lookup("tls-issuer")))
	checkErr(viper.BindEnv("tls-issuer", "TLS_ISSUER"))

	flags.String("access-control-allow-origin", "", "(ACCESS_CONTROL_ALLOW_ORIGIN) Domains allowed to use the API")
	checkErr(viper.BindPFlag("access-control-allow-origin", flags.Lookup("access-control-allow-origin")))
	checkErr(viper.BindEnv("access-control-allow-origin", "ACCESS_CONTROL_ALLOW_ORIGIN"))

	flags.String("registry-certificate-secret", "", "(REGISTRY_CERTIFICATE_SECRET) Secret for the registry's TLS certificate")
	checkErr(viper.BindPFlag("registry-certificate-secret", flags.Lookup("registry-certificate-secret")))
	checkErr(viper.BindEnv("registry-certificate-secret", "REGISTRY_CERTIFICATE_SECRET"))

	flags.String("s3-certificate-secret", "", "(S3_CERTIFICATE_SECRET) Secret for the S3 endpoint TLS certificate. Can be left empty if S3 is served with a trusted certificate.")
	checkErr(viper.BindPFlag("s3-certificate-secret", flags.Lookup("s3-certificate-secret")))
	checkErr(viper.BindEnv("s3-certificate-secret", "S3_CERTIFICATE_SECRET"))

	flags.String("ingress-class-name", "", "(INGRESS_CLASS_NAME) Name of the ingress class to use for apps. Leave empty to add no ingressClassName to the ingress.")
	checkErr(viper.BindPFlag("ingress-class-name", flags.Lookup("ingress-class-name")))
	checkErr(viper.BindEnv("ingress-class-name", "INGRESS_CLASS_NAME"))

	flags.String("app-image-exporter", "", "(APP_IMAGE_EXPORTER) Name of the container image used to download the application image from the 'export' API.")
	checkErr(viper.BindPFlag("app-image-exporter", flags.Lookup("app-image-exporter")))
	checkErr(viper.BindEnv("app-image-exporter", "APP_IMAGE_EXPORTER"))

	flags.String("default-builder-image", "", "(DEFAULT_BUILDER_IMAGE) Name of the container image used to build images from staged sources.")
	checkErr(viper.BindPFlag("default-builder-image", flags.Lookup("default-builder-image")))
	checkErr(viper.BindEnv("default-builder-image", "DEFAULT_BUILDER_IMAGE"))

	flags.Bool("disable-tracking", false, "(DISABLE_TRACKING) Disable tracking of the running Epinio and Kubernetes versions")
	checkErr(viper.BindPFlag("disable-tracking", flags.Lookup("disable-tracking")))
	checkErr(viper.BindEnv("disable-tracking", "DISABLE_TRACKING"))

	flags.String("upgrade-responder-address", upgraderesponder.UpgradeResponderAddress, "(UPGRADE_RESPONDER_ADDRESS) Address of the upgrade responder service")
	checkErr(viper.BindPFlag("upgrade-responder-address", flags.Lookup("upgrade-responder-address")))
	checkErr(viper.BindEnv("upgrade-responder-address", "UPGRADE_RESPONDER_ADDRESS"))

	flags.Float32("kube-api-qps", rest.DefaultQPS, "(KUBE_API_QPS) The QPS indicates the maximum QPS of the Kubernetes client.")
	checkErr(viper.BindPFlag("kube-api-qps", flags.Lookup("kube-api-qps")))
	checkErr(viper.BindEnv("kube-api-qps", "KUBE_API_QPS"))

	flags.Int("kube-api-burst", rest.DefaultBurst, "(KUBE_API_BURST) Maximum burst for throttle of the Kubernetes client.")
	checkErr(viper.BindPFlag("kube-api-burst", flags.Lookup("kube-api-burst")))
	checkErr(viper.BindEnv("kube-api-burst", "KUBE_API_BURST"))

	version.ChartVersion = os.Getenv("CHART_VERSION")
	if !strings.HasPrefix(version.ChartVersion, "v") {
		version.ChartVersion = "v" + version.ChartVersion
	}
}

// CmdServer implements the command: epinio server
var CmdServer = &cobra.Command{
	Use:   "server",
	Short: "Starts the Epinio server.",
	Long:  "This command starts the Epinio server. `epinio install` ensures the server is running inside your cluster. Normally you don't need to run this command manually.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		if helpers.Logger == nil {
			if err := helpers.InitLogger(viper.GetString("log-level")); err != nil {
				return errors.Wrap(err, "initializing logger")
			}
		}

		// NewHandler is defined in server.go (same package, moved from internal/cli/server/server.go)
		handler, err := NewHandler()
		if err != nil {
			return errors.Wrap(err, "error creating handler")
		}

		port := viper.GetInt("port")
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return errors.Wrap(err, "error creating listener")
		}

		helpers.Logger.Infow("Epinio version", "version", version.Version)
		listeningPort := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
		helpers.Logger.Infow("listening on localhost", "port", listeningPort)

		trackingDisabled := viper.GetBool("disable-tracking")
		upgradeResponderAddress := viper.GetString("upgrade-responder-address")
		helpers.Logger.Infow("checking upgrade-responder",
			"tracking_disabled", trackingDisabled,
			"upgrade_responder_address", upgradeResponderAddress,
		)

		if !trackingDisabled {
			logrLogger := helpers.LoggerToLogr().WithName("UpgradeResponder")
			checker, err := upgraderesponder.NewChecker(
				context.Background(),
				logrLogger,
				upgradeResponderAddress,
			)
			if err != nil {
				helpers.Logger.Errorw("error creating upgrade checker", "error", err)
				return err
			}

			checker.Start()
			defer checker.Stop()
		}

		return startServerGracefully(listener, handler)
	},
}

// Execute builds the minimal root command for the server binary and runs it.
func Execute() {
	rootCmd := &cobra.Command{
		Use:           "epinio",
		Short:         "Epinio server",
		Version:       version.Version,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return helpers.InitLogger(viper.GetString("log-level"))
		},
	}

	pf := rootCmd.PersistentFlags()

	pf.String("log-level", "info", "(LOG_LEVEL) Log level (debug, info, warn, error)")
	checkErr(viper.BindPFlag("log-level", pf.Lookup("log-level")))
	checkErr(viper.BindEnv("log-level", "LOG_LEVEL"))

	argToEnv := map[string]string{}
	duration.Flags(pf, argToEnv)

	rootCmd.AddCommand(CmdServer)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// startServerGracefully starts the HTTP server and blocks until SIGINT or SIGTERM.
func startServerGracefully(listener net.Listener, handler http.Handler) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attack
	}

	quit := make(chan os.Signal, 1)

	// In coverage mode we need to be able to terminate the server to collect the report.
	if _, ok := os.LookupEnv("GOCOVERDIR"); ok {
		router := handler.(*gin.Engine)
		router.GET("/exit", func(c *gin.Context) {
			c.AbortWithStatus(http.StatusNoContent)
			quit <- syscall.SIGTERM
		})
	}

	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			helpers.Logger.Errorw("server listen error", "error", err)
		}
	}()

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	helpers.Logger.Infow("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		helpers.Logger.Fatalw("Server forced to shutdown", "error", err)
		return err
	}

	helpers.Logger.Infow("Server exiting")
	return nil
}

// checkErr panics on viper binding errors during init. These are programmer
// errors (mismatched flag names) and should never occur at runtime.
func checkErr(err error) {
	if err != nil {
		if helpers.Logger != nil {
			helpers.Logger.Fatalw("fatal error", "error", err)
		}
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
