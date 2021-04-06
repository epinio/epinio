package cli

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suse/carrier/internal/cli/clients"
	"github.com/suse/carrier/termui"
)

var ()

// CmdOrgs implements the carrier orgs command
var CmdOrgs = &cobra.Command{
	Use:   "orgs",
	Short: "Lists all organizations",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Target remote carrier server instead of starting one
		port := viper.GetInt("port")
		httpServerWg := &sync.WaitGroup{}
		httpServerWg.Add(1)
		ui := termui.NewUI()
		srv, listeningPort, err := startCarrierServer(httpServerWg, port, ui)
		if err != nil {
			return err
		}

		// TODO: NOTE: This is a hack until the server is running inside the cluster
		cmd.Flags().String("server-url", "http://127.0.0.1:"+listeningPort, "")

		client, err := clients.NewCarrierClient(cmd.Flags())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.Orgs()
		if err != nil {
			return errors.Wrap(err, "error listing orgs")
		}

		if err := srv.Shutdown(context.Background()); err != nil {
			return err
		}
		httpServerWg.Wait()

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}
