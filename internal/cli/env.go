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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// CmdAppEnv implements the command: epinio app env
var CmdAppEnv = &cobra.Command{
	Use:   "env",
	Short: "Epinio application configuration",
	Long:  `Manage epinio application environment variables`,
}

func init() {
	CmdAppEnv.AddCommand(CmdEnvList)
	CmdAppEnv.AddCommand(CmdEnvSet)
	CmdAppEnv.AddCommand(CmdEnvShow)
	CmdAppEnv.AddCommand(CmdEnvUnset)
}

// CmdEnvList implements the command: epinio app env list
var CmdEnvList = &cobra.Command{
	Use:               "list APPNAME",
	Short:             "Lists application environment",
	Long:              "Lists environment variables of named application",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: matchingAppsFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		err := client.EnvList(cmd.Context(), args[0])
		if err != nil {
			return errors.Wrap(err, "error listing app environment")
		}

		return nil
	},
}

// CmdEnvSet implements the command: epinio app env set
var CmdEnvSet = &cobra.Command{
	Use:               "set APPNAME NAME VALUE",
	Short:             "Extend application environment",
	Long:              "Add or change environment variable of named application",
	Args:              cobra.ExactArgs(3),
	ValidArgsFunction: matchingAppsFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		err := client.EnvSet(cmd.Context(), args[0], args[1], args[2])
		if err != nil {
			return errors.Wrap(err, "error setting into app environment")
		}

		return nil
	},
}

// CmdEnvShow implements the command: epinio app env show
var CmdEnvShow = &cobra.Command{
	Use:   "show APPNAME NAME",
	Short: "Describe application's environment variable",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		err := client.EnvShow(cmd.Context(), args[0], args[1])
		if err != nil {
			return errors.Wrap(err, "error accessing app environment")
		}

		return nil
	},
	ValidArgsFunction: matchingAppAndVarFinder,
}

// CmdEnvUnset implements the command: epinio app env unset
var CmdEnvUnset = &cobra.Command{
	Use:   "unset APPNAME NAME",
	Short: "Shrink application environment",
	Long:  "Remove environment variable from named application",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		err := client.EnvUnset(cmd.Context(), args[0], args[1])
		if err != nil {
			return errors.Wrap(err, "error removing from app environment")
		}

		return nil
	},
	ValidArgsFunction: matchingAppAndVarFinder,
}

func matchingAppAndVarFinder(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// #args == 2, 3, ... nothing matches
	if len(args) > 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client.API.DisableVersionWarning()

	if len(args) == 1 {
		// #args == 1: environment variable name (in application)
		matches := client.EnvMatching(cmd.Context(), args[0], toComplete)
		return matches, cobra.ShellCompDirectiveNoFileComp
	}

	// #args == 0: application name.
	matches := client.AppsMatching(toComplete)

	return matches, cobra.ShellCompDirectiveNoFileComp
}
