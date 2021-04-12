package cli

import (
	"context"
	"sync"

	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/termui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var ()

// CmdOrgCreate implements the epinio `orgs create` command
var CmdOrgCreate = &cobra.Command{
	Use:   "create NAME",
	Short: "Creates an organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Target remote epinio server instead of starting one
		port := viper.GetInt("port")
		httpServerWg := &sync.WaitGroup{}
		httpServerWg.Add(1)
		ui := termui.NewUI()
		srv, listeningPort, err := startEpinioServer(httpServerWg, port, ui)
		if err != nil {
			return err
		}

		// TODO: NOTE: This is a hack until the server is running inside the cluster
		cmd.Flags().String("server-url", "http://127.0.0.1:"+listeningPort, "")

		client, err := clients.NewEpinioClient(cmd.Flags())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.CreateOrg(args[0])
		if err != nil {
			return errors.Wrap(err, "error creating org")
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
