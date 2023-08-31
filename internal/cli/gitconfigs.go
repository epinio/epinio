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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	gGitconfigAllFlag bool
)

// CmdGitconfig implements the command: epinio gitconfig
var CmdGitconfig = &cobra.Command{
	Use:           "gitconfig",
	Aliases:       []string{"gitconfigs"},
	Short:         "Epinio git configuration management",
	Long:          `Manage git configurations`,
	SilenceErrors: true,
	SilenceUsage:  true,
	Args:          cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cmd.Usage(); err != nil {
			return err
		}
		return fmt.Errorf(`Unknown method "%s"`, args[0])
	},
}

func init() {
	flags := CmdGitconfigDelete.Flags()
	flags.BoolVar(&gGitconfigAllFlag, "all", false, "delete all git configurations")

	gitProviderOption(CmdGitconfigCreate)
	flags = CmdGitconfigCreate.Flags()
	flags.Bool("skip-ssl", false, "skip ssl")
	flags.String("username", "", "user name for logging into the host")
	flags.String("password", "", "password for logging into the host")
	flags.String("user-org", "", "user/org holding repository")
	flags.String("repository", "", "specific repository")
	flags.String("cert-file", "", "path to file holding supporting certificates")

	CmdGitconfig.AddCommand(CmdGitconfigCreate)
	CmdGitconfig.AddCommand(CmdGitconfigList)
	CmdGitconfig.AddCommand(CmdGitconfigDelete)
	CmdGitconfig.AddCommand(CmdGitconfigShow)
}

// CmdGitconfigs implements the command: epinio gitconfig list
var CmdGitconfigList = &cobra.Command{
	Use:   "list",
	Short: "Lists all git configurations",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		err := client.Gitconfigs()
		if err != nil {
			return errors.Wrap(err, "error listing git configurations")
		}

		return nil
	},
}

// CmdGitconfigCreate implements the command: epinio gitconfig create
var CmdGitconfigCreate = &cobra.Command{
	Use:   "create ID URL [flags]",
	Short: "Creates a git configuration",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		skipssl, err := cmd.Flags().GetBool("skip-ssl")
		if err != nil {
			return errors.Wrap(err, "could not read option --skip-ssl")
		}
		provider, err := cmd.Flags().GetString("git-provider")
		if err != nil {
			return errors.Wrap(err, "could not read option --git-provider")
		}
		if provider == "" {
			provider = "git"
		}
		user, err := cmd.Flags().GetString("username")
		if err != nil {
			return errors.Wrap(err, "could not read option --username")
		}
		password, err := cmd.Flags().GetString("password")
		if err != nil {
			return errors.Wrap(err, "could not read option --password")
		}
		userorg, err := cmd.Flags().GetString("user-org")
		if err != nil {
			return errors.Wrap(err, "could not read option --user-org")
		}
		repository, err := cmd.Flags().GetString("repository")
		if err != nil {
			return errors.Wrap(err, "could not read option --repository")
		}
		certfile, err := cmd.Flags().GetString("cert-file")
		if err != nil {
			return errors.Wrap(err, "could not read option --cert-file")
		}

		id := args[0]
		url := args[1]

		err = client.CreateGitconfig(id, provider, url,
			user, password, userorg, repository, certfile, skipssl)
		if err != nil {
			return errors.Wrap(err, "error creating git configuration")
		}

		return nil
	},
}

// CmdGitconfigDelete implements the command: epinio gitconfig delete
var CmdGitconfigDelete = &cobra.Command{
	Use:               "delete NAME",
	Short:             "Delete git configurations",
	ValidArgsFunction: matchingGitconfigFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		err := client.DeleteGitconfig(args, gGitconfigAllFlag)
		if err != nil {
			// Cancellation is not an "error" in deletion.
			if !strings.Contains(err.Error(), "Cancelled") {
				err = errors.Wrap(err, "error deleting git configuration")
			}
			return err
		}

		return nil
	},
}

// CmdGitconfigShow implements the command: epinio gitconfig show
var CmdGitconfigShow = &cobra.Command{
	Use:               "show NAME",
	Short:             "Shows the details of a git configuration",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: matchingGitconfigFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		err := client.ShowGitconfig(args[0])
		if err != nil {
			return errors.Wrap(err, "error showing git configuration")
		}

		return nil
	},
}
