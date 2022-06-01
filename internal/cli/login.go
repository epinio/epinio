package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/spf13/cobra"
)

func init() {
	CmdLogin.Flags().StringP("user", "u", "", "username that will be used to login")
	CmdLogin.Flags().StringP("password", "p", "", "password that will be used to login")
	CmdLogin.Flags().Bool("trust-ca", false, "set this flag to automatically trust the unknown CA")
}

// CmdLogin implements the command: epinio login
var CmdLogin = &cobra.Command{
	Use:   "login [URL]",
	Short: "Epinio login to the server",
	Long:  `The login command will setup the settings file with the provided credentials for the specified Epinio cluster`,
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

		return client.Login(username, password, address, trustCA)
	},
}
