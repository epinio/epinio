package cli

import (
	"sync"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/server"

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

	flags.Bool("force-kube-internal-registry-tls", false, "(FORCE_KUBE_INTERNAL_REGISTRY_TLS) Kubernetes accesses the internal registry over TLS")
	viper.BindPFlag("force-kube-internal-registry-tls", flags.Lookup("force-kube-internal-registry-tls"))
	viper.BindEnv("force-kube-internal-registry-tls", "FORCE_KUBE_INTERNAL_REGISTRY_TLS")

	flags.String("access-control-allow-origin", "", "(ACCESS_CONTROL_ALLOW_ORIGIN) Domains allowed to use the API")
	viper.BindPFlag("access-control-allow-origin", flags.Lookup("access-control-allow-origin"))
	viper.BindEnv("access-control-allow-origin", "ACCESS_CONTROL_ALLOW_ORIGIN")
}

// CmdServer implements the command: epinio server
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
		logger := tracelog.NewLogger().WithName("EpinioServer")
		_, listeningPort, err := server.Start(httpServerWg, port, ui, logger)
		if err != nil {
			return errors.Wrap(err, "failed to start server")
		}
		ui.Normal().Msg("listening on localhost on port " + listeningPort)
		httpServerWg.Wait()

		return nil
	},
}
