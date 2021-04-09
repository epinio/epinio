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

// CmdOrg implements the carrier -app command
var CmdOrg = &cobra.Command{
	Use:           "org",
	Aliases:       []string{"orgs"},
	Short:         "Carrier organizations",
	Long:          `Manage carrier organizations`,
	Args:          cobra.ExactArgs(0),
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	CmdOrg.AddCommand(CmdOrgCreate)
	CmdOrg.AddCommand(CmdOrgList)
}

// CmdOrgs implements the carrier `orgs list` command
var CmdOrgList = &cobra.Command{
	Use:   "list",
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
