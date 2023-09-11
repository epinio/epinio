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

package cmd

import (
	"context"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

//counterfeiter:generate -header ../../../LICENSE_HEADER . LoginService
type LoginService interface {
	LoginOIDC(ctx context.Context, address string, trustCA, prompt bool) error
	Login(ctx context.Context, username, password, address string, trustCA bool) error
	Logout(ctx context.Context) error
}

type LoginConfig struct {
	user     string
	password string
	trustCA  bool
	oidc     bool
	prompt   bool
}

// NewLoginCmd returns a new 'epinio login' command
func NewLoginCmd(client LoginService) *cobra.Command {
	cfg := LoginConfig{}

	loginCmd := &cobra.Command{
		Use:   "login [URL]",
		Short: "Epinio login to the server",
		Long:  `The login command allows you to authenticate against an Epinio instance and updates the settings file with the generated authentication token`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			address, err := validateAddress(args[0])
			if err != nil {
				return err
			}

			if cfg.oidc {
				return client.LoginOIDC(cmd.Context(), address, cfg.trustCA, cfg.prompt)
			}
			return client.Login(cmd.Context(), cfg.user, cfg.password, address, cfg.trustCA)
		},
	}

	loginCmd.Flags().StringVarP(&cfg.user, "user", "u", "", "username that will be used to login")
	loginCmd.Flags().StringVarP(&cfg.password, "password", "p", "", "password that will be used to login")
	loginCmd.Flags().BoolVar(&cfg.trustCA, "trust-ca", false, "automatically trust the unknown CA")
	loginCmd.Flags().BoolVar(&cfg.oidc, "oidc", false, "perform OIDC authentication (user and password will be ignored)")
	loginCmd.Flags().BoolVar(&cfg.prompt, "prompt", false, "enable the prompt of the authorization code and disable the local server during OIDC authentication")

	return loginCmd
}

// NewLogoutCmd returns a new 'epinio logout' command
func NewLogoutCmd(client LoginService) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Epinio logout from server",
		Long:  `The logout command removs all authentication information from the local state, i.e. settings file`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			return client.Logout(cmd.Context())
		},
	}
}

// validateAddress will validate the provided address falling back to https schema if none
func validateAddress(address string) (string, error) {
	parsedURL, err := url.Parse(address)
	if err != nil {
		return "", err
	}

	// if the scheme is missing fallback to https
	if parsedURL.Scheme == "" {
		address = fmt.Sprintf("https://%s", address)
	}

	return address, nil
}
