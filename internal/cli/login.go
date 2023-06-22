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
	"github.com/spf13/cobra"
)

func init() {
	CmdLogin.Flags().StringP("user", "u", "", "username that will be used to login")
	CmdLogin.Flags().StringP("password", "p", "", "password that will be used to login")
	CmdLogin.Flags().Bool("trust-ca", false, "automatically trust the unknown CA")
	CmdLogin.Flags().Bool("oidc", false, "perform OIDC authentication (user and password will be ignored)")
	CmdLogin.Flags().Bool("prompt", false, "enable the prompt of the authorization code and disable the local server during OIDC authentication")
}

// CmdLogout implements the command: epinio logout
var CmdLogout = &cobra.Command{
	Use:   "logout",
	Short: "Epinio logout from server",
	Long:  `The logout command removs all authentication information from the local state, i.e. settings file`,
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		return client.Logout(cmd.Context())
	},
}

// CmdLogin implements the command: epinio login
var CmdLogin = &cobra.Command{
	Use:   "login [URL]",
	Short: "Epinio login to the server",
	Long:  `The login command allows you to authenticate against an Epinio instance and updates the settings file with the generated authentication token`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

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
		return client.Login(cmd.Context(), username, password, address, trustCA)
	},
}
