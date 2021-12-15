package cli

import (
	"fmt"
	"sync"

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

	flags.String("tls-issuer", "", "(TLS_ISSUER) The cluster issuer to use for workload certificates")
	viper.BindPFlag("tls-issuer", flags.Lookup("tls-issuer"))
	viper.BindEnv("tls-issuer", "TLS_ISSUER")

	flags.String("access-control-allow-origin", "", "(ACCESS_CONTROL_ALLOW_ORIGIN) Domains allowed to use the API")
	viper.BindPFlag("access-control-allow-origin", flags.Lookup("access-control-allow-origin"))
	viper.BindEnv("access-control-allow-origin", "ACCESS_CONTROL_ALLOW_ORIGIN")

	flags.String("registry-certificate-secret", "", "(REGISTRY_CERTIFICATE_SECRET) Secret for the registry's TLS certificate")
	viper.BindPFlag("registry-certificate-secret", flags.Lookup("registry-certificate-secret"))
	viper.BindEnv("registry-certificate-secret", "REGISTRY_CERTIFICATE_SECRET")
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
		eventsChan := make(chan map[string]string)
		_, listeningPort, err := server.Start(httpServerWg, port, ui, eventsChan, logger)
		if err != nil {
			return errors.Wrap(err, "failed to start server")
		}
		ui.Normal().Msg("listening on localhost on port " + listeningPort)

		httpServerWg.Add(1)
		go func(chan map[string]string) {
			defer httpServerWg.Done()
			for e := range eventsChan {
				// TODO: Instead of just printing the event, we need some logic here
				// that decides which subscribers should receive this event.
				// We also need a way to add subscribers. This can be another channel
				// over which subscribers wil be sent (from the "/subscribe" endpoint).
				// Each subscriber should have a websocket connection to which we should
				// send the event.
				fmt.Println(e)
			}
		}(eventsChan)

		httpServerWg.Wait()

		return nil
	},
}
