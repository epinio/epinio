package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/spf13/cobra"
)

func init() {
	CmdLogin.Flags().Bool("trust-ca", false, "set this flag to automatically trust the unknown CA")
}

// CmdLogin implements the command: epinio login
// It implements the "public client" flow of dex:
// https://dexidp.io/docs/custom-scopes-claims-clients/#public-clients
// https://github.com/dexidp/dex/issues/469
// https://developers.google.com/identity/protocols/oauth2/native-app
var CmdLogin = &cobra.Command{
	Use:   "login [URL]",
	Short: "Epinio login to the server",
	Long:  `The login command allows you to authenticate against an Epinio instance and updates the settings file with the generated authentication token`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return err
		}

		address := args[0]

		trustCA, err := cmd.Flags().GetBool("trust-ca")
		if err != nil {
			return err
		}

		return client.Login(cmd.Context(), address, trustCA)
	},
}
