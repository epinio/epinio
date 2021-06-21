package cli

import (
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/internal/cli/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

// CmdConfig implements the `epinio config` command
var CmdConfig = &cobra.Command{
	Use:           "config",
	Short:         "Epinio config management",
	Long:          `Manage the epinio cli configuration`,
	Args:          cobra.ExactArgs(0),
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	CmdConfig.AddCommand(CmdConfigUpdateCreds)
	CmdConfig.AddCommand(CmdConfigShow)
}

// CmdConfigShow implements the `epinio config show` command
var CmdConfigShow = &cobra.Command{
	Use:   "show",
	Short: "Show the current configuration",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		ui := termui.NewUI()

		theConfig, err := config.Load()
		if err != nil {
			return err
		}

		certInfo := "None defined"
		if theConfig.Certs != "" {
			certInfo = "Present"
		}

		ui.Success().
			WithTable("Key", "Value").
			WithTableRow("Current Organization", theConfig.Org).
			WithTableRow("API User Name", theConfig.User).
			WithTableRow("API Password", theConfig.Password).
			WithTableRow("Certificates", certInfo).
			Msg("Ok")

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

// CmdConfigUpdateCreds implements the `epinio config update-credentials` command
var CmdConfigUpdateCreds = &cobra.Command{
	Use:   "update-credentials",
	Short: "Update the stored credentials",
	Long:  "Update the stored credentials from the current cluster",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clients.NewEpinioClient(cmd.Context(), cmd.Flags())

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.ConfigUpdate(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error updating the config")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}
