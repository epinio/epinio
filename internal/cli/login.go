package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/spf13/cobra"
)

func init() {
	CmdLogin.Flags().StringP("user", "u", "", "username that will be used to login")
	CmdLogin.Flags().StringP("password", "p", "", "password that will be used to login")
	CmdLogin.Flags().Bool("trust-ca", false, "automatically trust the unknown CA")
	CmdLogin.Flags().Bool("oidc", false, "perform OIDC authentication (user and password will be ignored)")
	CmdLogin.Flags().Bool("prompt", false, "enable the prompt of the authorization code and disable the local server during OIDC authentication")
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
		username, err := cmd.Flags().GetString("user")
		if err != nil {
			return err
		}

		password, err := cmd.Flags().GetString("password")
		if err != nil {
			return err
		}

		trustCA, err := cmd.Flags().GetBool("trust-ca")
		if err != nil {
			return err
		}

		oidc, err := cmd.Flags().GetBool("oidc")
		if err != nil {
			return err
		}

		prompt, err := cmd.Flags().GetBool("prompt")
		if err != nil {
			return err
		}

		if oidc {
			return client.LoginOIDC(cmd.Context(), address, trustCA, prompt)
		}
		return client.Login(username, password, address, trustCA)
	},
}
