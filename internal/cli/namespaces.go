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
	gForceFlag bool
	gAllFlag   bool
)

// CmdNamespace implements the command: epinio namespace
var CmdNamespace = &cobra.Command{
	Use:           "namespace",
	Aliases:       []string{"namespaces"},
	Short:         "Epinio-controlled namespaces",
	Long:          `Manage epinio-controlled namespaces`,
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

	flags := CmdNamespaceDelete.Flags()
	flags.BoolVarP(&gForceFlag, "force", "f", false, "force namespace deletion")
	flags.BoolVar(&gAllFlag, "all", false, "delete all namespaces")

	CmdNamespace.AddCommand(CmdNamespaceCreate)
	CmdNamespace.AddCommand(CmdNamespaceList)
	CmdNamespace.AddCommand(CmdNamespaceDelete)
	CmdNamespace.AddCommand(CmdNamespaceShow)
}

// CmdNamespaces implements the command: epinio namespace list
var CmdNamespaceList = &cobra.Command{
	Use:   "list",
	Short: "Lists all epinio-controlled namespaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		err := client.Namespaces()
		if err != nil {
			return errors.Wrap(err, "error listing epinio-controlled namespaces")
		}

		return nil
	},
}

// CmdNamespaceCreate implements the command: epinio namespace create
var CmdNamespaceCreate = &cobra.Command{
	Use:   "create NAME",
	Short: "Creates an epinio-controlled namespace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		err := client.CreateNamespace(args[0])
		if err != nil {
			return errors.Wrap(err, "error creating epinio-controlled namespace")
		}

		return nil
	},
}

// CmdNamespaceDelete implements the command: epinio namespace delete
var CmdNamespaceDelete = &cobra.Command{
	Use:               "delete NAME",
	Short:             "Deletes an epinio-controlled namespace",
	ValidArgsFunction: matchingNamespaceFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		err := client.DeleteNamespace(args, gForceFlag, gAllFlag)
		if err != nil {
			// Cancellation is not an "error" in deletion.
			if !strings.Contains(err.Error(), "Cancelled") {
				err = errors.Wrap(err, "error deleting epinio-controlled namespace")
			}
			return err
		}

		return nil
	},
}

// CmdNamespaceShow implements the command: epinio namespace show
var CmdNamespaceShow = &cobra.Command{
	Use:               "show NAME",
	Short:             "Shows the details of an epinio-controlled namespace",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: matchingNamespaceFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		err := client.ShowNamespace(args[0])
		if err != nil {
			return errors.Wrap(err, "error showing epinio-controlled namespace")
		}

		return nil
	},
}
