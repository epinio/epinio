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
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/server"
	"github.com/epinio/epinio/internal/cli/termui"
	"github.com/epinio/epinio/internal/upgraderesponder"
	"github.com/epinio/epinio/internal/version"
	"github.com/gin-gonic/gin"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/rest"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

	flags.String("staging-service-account-name", "", "(STAGING_SERVICE_ACCOUNT_NAME)")
	err = viper.BindPFlag("staging-service-account-name", flags.Lookup("staging-service-account-name"))
	checkErr(err)
	err = viper.BindEnv("staging-service-account-name", "STAGING_SERVICE_ACCOUNT_NAME")
	checkErr(err)

	flags.String("staging-resource-cpu", "", "(STAGING_RESOURCE_CPU)")
	err = viper.BindPFlag("staging-resource-cpu", flags.Lookup("staging-resource-cpu"))
	checkErr(err)
	err = viper.BindEnv("staging-resource-cpu", "STAGING_RESOURCE_CPU")
	checkErr(err)

	flags.String("staging-resource-memory", "", "(STAGING_RESOURCE_MEMORY)")
	err = viper.BindPFlag("staging-resource-memory", flags.Lookup("staging-resource-memory"))
	checkErr(err)
	err = viper.BindEnv("staging-resource-memory", "STAGING_RESOURCE_MEMORY")
	checkErr(err)

	flags.String("staging-resource-disk", "", "(STAGING_RESOURCE_DISK)")
	err = viper.BindPFlag("staging-resource-disk", flags.Lookup("staging-resource-disk"))
	checkErr(err)
	err = viper.BindEnv("staging-resource-disk", "STAGING_RESOURCE_DISK")
	checkErr(err)

	flags.String("upgrade-responder-address", upgraderesponder.UpgradeResponderAddress, "(UPGRADE_RESPONDER_ADDRESS) Disable tracking of the running Epinio and Kubernetes versions")
	err = viper.BindPFlag("upgrade-responder-address", flags.Lookup("upgrade-responder-address"))
	checkErr(err)
	err = viper.BindEnv("upgrade-responder-address", "UPGRADE_RESPONDER_ADDRESS")
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
		logger := tracelog.NewLogger().WithName("EpinioServer")

		// Validate resource requests for staging job here, on server startup, to reject bad values immediately.

		cpuRequest := viper.GetString("staging-resource-cpu")
		if cpuRequest != "" {
			_, err := resource.ParseQuantity(cpuRequest)
			if err != nil {
				return errors.Wrap(err, "bad cpu request for staging job")
			}
		}
		memoryRequest := viper.GetString("staging-resource-memory")
		if memoryRequest != "" {
			_, err := resource.ParseQuantity(memoryRequest)
			if err != nil {
				return errors.Wrap(err, "bad memory request for staging job")
			}
		}
		diskRequest := viper.GetString("staging-resource-disk")
		if diskRequest != "" {
			_, err := resource.ParseQuantity(diskRequest)
			if err != nil {
				return errors.Wrap(err, "bad disk size request for staging job")
			}
		}

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

		trackingDisabled := viper.GetBool("disable-tracking")
		upgradeResponderAddress := viper.GetString("upgrade-responder-address")
		logger.Info("Checking upgrade-responder tracking", "disabled", trackingDisabled, "upgradeResponderAddress", upgradeResponderAddress)

		if !trackingDisabled {
			checker, err := upgraderesponder.NewChecker(context.Background(), logger, upgradeResponderAddress)
			if err != nil {
				return errors.Wrap(err, "error creating listener")
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
		if err := srv.Serve(listener); err != nil && errors.Is(err, http.ErrServerClosed) {
			log.Printf("listen: %s\n", err)
		}
	}()

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
