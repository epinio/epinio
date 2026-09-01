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

package cli

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
	"github.com/epinio/epinio/internal/cli/server"
	"github.com/epinio/epinio/internal/upgraderesponder"
	"github.com/epinio/epinio/internal/version"
	"github.com/gin-gonic/gin"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/rest"
)

func init() {
	flags := CmdServer.Flags()

	flags.StringP("namespace", "n", "epinio", "(NAMESPACE) The namespace to use")
	err := viper.BindPFlag("namespace", flags.Lookup("namespace"))
	checkErr(err)
	err = viper.BindEnv("namespace", "NAMESPACE")
	checkErr(err)

	flags.Int("port", 0, "(PORT) The port to listen on. Leave empty to auto-assign a random port")
	err = viper.BindPFlag("port", flags.Lookup("port"))
	checkErr(err)
	err = viper.BindEnv("port", "PORT")
	checkErr(err)

	flags.String("tls-issuer", "", "(TLS_ISSUER) The cluster issuer to use for workload certificates")
	err = viper.BindPFlag("tls-issuer", flags.Lookup("tls-issuer"))
	checkErr(err)
	err = viper.BindEnv("tls-issuer", "TLS_ISSUER")
	checkErr(err)

	flags.String("access-control-allow-origin", "", "(ACCESS_CONTROL_ALLOW_ORIGIN) Domains allowed to use the API")
	err = viper.BindPFlag("access-control-allow-origin", flags.Lookup("access-control-allow-origin"))
	checkErr(err)
	err = viper.BindEnv("access-control-allow-origin", "ACCESS_CONTROL_ALLOW_ORIGIN")
	checkErr(err)

	flags.String("registry-certificate-secret", "", "(REGISTRY_CERTIFICATE_SECRET) Secret for the registry's TLS certificate")
	err = viper.BindPFlag("registry-certificate-secret", flags.Lookup("registry-certificate-secret"))
	checkErr(err)
	err = viper.BindEnv("registry-certificate-secret", "REGISTRY_CERTIFICATE_SECRET")
	checkErr(err)

	flags.String("s3-certificate-secret", "", "(S3_CERTIFICATE_SECRET) Secret for the S3 endpoint TLS certificate. Can be left empty if S3 is served with a trusted certificate.")
	err = viper.BindPFlag("s3-certificate-secret", flags.Lookup("s3-certificate-secret"))
	checkErr(err)
	err = viper.BindEnv("s3-certificate-secret", "S3_CERTIFICATE_SECRET")
	checkErr(err)

	flags.String("ingress-class-name", "", "(INGRESS_CLASS_NAME) Name of the ingress class to use for apps. Leave empty to add no ingressClassName to the ingress.")
	err = viper.BindPFlag("ingress-class-name", flags.Lookup("ingress-class-name"))
	checkErr(err)
	err = viper.BindEnv("ingress-class-name", "INGRESS_CLASS_NAME")
	checkErr(err)

	flags.String("gateway-class-name", "", "(GATEWAY_CLASS_NAME) Name of the gateway class to use for apps. Leave empty to add no gatewayClassName to the gateway.")
	err = viper.BindPFlag("gateway-class-name", flags.Lookup("gateway-class-name"))
	checkErr(err)
	err = viper.BindEnv("gateway-class-name", "GATEWAY_CLASS_NAME")
	checkErr(err)

	flags.String("app-image-exporter", "", "(APP_IMAGE_EXPORTER) Name of the container image used to download the application image from the 'export' API.")
	err = viper.BindPFlag("app-image-exporter", flags.Lookup("app-image-exporter"))
	checkErr(err)
	err = viper.BindEnv("app-image-exporter", "APP_IMAGE_EXPORTER")
	checkErr(err)

	flags.String("default-builder-image", "", "(DEFAULT_BUILDER_IMAGE) Name of the container image used to build images from staged sources.")
	err = viper.BindPFlag("default-builder-image", flags.Lookup("default-builder-image"))
	checkErr(err)
	err = viper.BindEnv("default-builder-image", "DEFAULT_BUILDER_IMAGE")
	checkErr(err)

	flags.Bool("disable-tracking", false, "(DISABLE_TRACKING) Disable tracking of the running Epinio and Kubernetes versions")
	err = viper.BindPFlag("disable-tracking", flags.Lookup("disable-tracking"))
	checkErr(err)
	err = viper.BindEnv("disable-tracking", "DISABLE_TRACKING")
	checkErr(err)

	flags.String("upgrade-responder-address", upgraderesponder.UpgradeResponderAddress, "(UPGRADE_RESPONDER_ADDRESS) Disable tracking of the running Epinio and Kubernetes versions")
	err = viper.BindPFlag("upgrade-responder-address", flags.Lookup("upgrade-responder-address"))
	checkErr(err)
	err = viper.BindEnv("upgrade-responder-address", "UPGRADE_RESPONDER_ADDRESS")
	checkErr(err)

	flags.String("install-method", "helm", "(INSTALL_METHOD) How this Epinio instance was installed (helm|cli). Used when creating a missing instance id. Defaults to helm; CLI/installer wrappers should set INSTALL_METHOD=cli.")
	err = viper.BindPFlag("install-method", flags.Lookup("install-method"))
	checkErr(err)
	err = viper.BindEnv("install-method", "INSTALL_METHOD")
	checkErr(err)

	flags.Float32("kube-api-qps", rest.DefaultQPS, "(KUBE_API_QPS) The QPS indicates the maximum QPS of the Kubernetes client.")
	err = viper.BindPFlag("kube-api-qps", flags.Lookup("kube-api-qps"))
	checkErr(err)
	err = viper.BindEnv("kube-api-qps", "KUBE_API_QPS")
	checkErr(err)

	flags.Int("kube-api-burst", rest.DefaultBurst, "(KUBE_API_BURST) Maximum burst for throttle of the Kubernetes client.")
	err = viper.BindPFlag("kube-api-burst", flags.Lookup("kube-api-burst"))
	checkErr(err)
	err = viper.BindEnv("kube-api-burst", "KUBE_API_BURST")
	checkErr(err)

	flags.Bool("telemetry-enabled", true, "(TELEMETRY_ENABLED) Enable the daily fleet telemetry push to Grafana Cloud")
	err = viper.BindPFlag("telemetry-enabled", flags.Lookup("telemetry-enabled"))
	checkErr(err)
	err = viper.BindEnv("telemetry-enabled", "TELEMETRY_ENABLED")
	checkErr(err)

	flags.String("telemetry-otlp-endpoint", "", "(TELEMETRY_OTLP_ENDPOINT) Grafana Cloud OTLP/HTTP metrics endpoint")
	err = viper.BindPFlag("telemetry-otlp-endpoint", flags.Lookup("telemetry-otlp-endpoint"))
	checkErr(err)
	err = viper.BindEnv("telemetry-otlp-endpoint", "TELEMETRY_OTLP_ENDPOINT")
	checkErr(err)

	flags.String("telemetry-grafana-instance-id", "", "(TELEMETRY_GRAFANA_INSTANCE_ID) Grafana Cloud stack instance ID, used as the OTLP Basic auth username")
	err = viper.BindPFlag("telemetry-grafana-instance-id", flags.Lookup("telemetry-grafana-instance-id"))
	checkErr(err)
	err = viper.BindEnv("telemetry-grafana-instance-id", "TELEMETRY_GRAFANA_INSTANCE_ID")
	checkErr(err)

	flags.String("telemetry-grafana-token", "", "(TELEMETRY_GRAFANA_TOKEN) Grafana Cloud access-policy token, used as the OTLP Basic auth password")
	err = viper.BindPFlag("telemetry-grafana-token", flags.Lookup("telemetry-grafana-token"))
	checkErr(err)
	err = viper.BindEnv("telemetry-grafana-token", "TELEMETRY_GRAFANA_TOKEN")
	checkErr(err)

	flags.String("telemetry-trigger-token", "", "(TELEMETRY_TRIGGER_TOKEN) Shared secret the epinio-telemetry CronJob must send to trigger a telemetry push")
	err = viper.BindPFlag("telemetry-trigger-token", flags.Lookup("telemetry-trigger-token"))
	checkErr(err)
	err = viper.BindEnv("telemetry-trigger-token", "TELEMETRY_TRIGGER_TOKEN")
	checkErr(err)

	flags.String("telemetry-cluster-label", "", "(TELEMETRY_CLUSTER_LABEL) Optional cluster label attached to pushed telemetry metrics")
	err = viper.BindPFlag("telemetry-cluster-label", flags.Lookup("telemetry-cluster-label"))
	checkErr(err)
	err = viper.BindEnv("telemetry-cluster-label", "TELEMETRY_CLUSTER_LABEL")
	checkErr(err)

	flags.String("telemetry-environment-label", "", "(TELEMETRY_ENVIRONMENT_LABEL) Optional environment label attached to pushed telemetry metrics")
	err = viper.BindPFlag("telemetry-environment-label", flags.Lookup("telemetry-environment-label"))
	checkErr(err)
	err = viper.BindEnv("telemetry-environment-label", "TELEMETRY_ENVIRONMENT_LABEL")
	checkErr(err)

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
		// Ensure the centralized logger is initialized (root persistent pre-run normally does this)
		if helpers.Logger == nil {
			if err := helpers.InitLogger(viper.GetString("log-level")); err != nil {
				return errors.Wrap(err, "initializing logger")
			}
		}

		handler, err := server.NewHandler()
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
			// Convert zap logger to logr.Logger for upgraderesponder (compatibility bridge)
			logrLogger := helpers.LoggerToLogr().WithName("UpgradeResponder")
			checker, err := upgraderesponder.NewChecker(
				context.Background(),
				logrLogger,
				upgradeResponderAddress,
			)

			if err != nil {
				helpers.Logger.Errorw("error creating listener", "error", err)
				return err
			}

			checker.Start()
			defer checker.Stop()
		}

		return startServerGracefully(listener, handler)
	},
}

// startServerGracefully will start the server and will wait for a graceful shutdown
func startServerGracefully(listener net.Listener, handler http.Handler) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attack
	}

	quit := make(chan os.Signal, 1)

	// in coverage mode we need to be able to terminate the server to collect the report
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
