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
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	force bool
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
	flags.BoolVarP(&force, "force", "f", false, "force namespace deletion")
	flags.Bool("all", false, "delete all namespaces")

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

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.Namespaces()
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

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.CreateNamespace(args[0])
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
	Args:              cobra.MinimumNArgs(0),
	ValidArgsFunction: matchingNamespaceFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return errors.Wrap(err, "error reading option --all")
		}

		if all && len(args) > 0 {
			return errors.New("Conflict between --all and given namespaces")
		}
		if !all && len(args) == 0 {
			return errors.New("No namespaces specified for deletion")
		}

		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return err
		}

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		if !force {
			if all {
				resp, err := client.API.NamespacesMatch("")
				if err != nil {
					return err
				}

				args = resp.Names
			}

			if len(args) == 1 {
				cmd.Printf("You are about to delete the namespace '%s' and everything\nit includes, i.e. applications, configurations, etc.\nAre you sure? (y/n): ",
					args[0])
			} else {
				names := strings.Join(args, ", ")
				cmd.Printf("You are about to delete %d namespaces (%s)\nand everything they include,i.e. applications, configurations, etc.\nAre you sure? (y/n): ",
					len(args), names)
			}

			if !askConfirmation(cmd) {
				return errors.New("Cancelled by user")
			}
		}

		err = client.DeleteNamespace(args, all)
		if err != nil {
			return errors.Wrap(err, "error deleting epinio-controlled namespace")
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

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.ShowNamespace(args[0])
		if err != nil {
			return errors.Wrap(err, "error showing epinio-controlled namespace")
		}

		return nil
	},
}

// askConfirmation is a helper for CmdNamespaceDelete to confirm a deletion request
func askConfirmation(cmd *cobra.Command) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		s, _ := reader.ReadString('\n')
		s = strings.TrimSpace(strings.ToLower(s))
		if strings.Compare(s, "n") == 0 {
			return false
		} else if strings.Compare(s, "y") == 0 {
			break
		} else {
			cmd.Printf("Please enter y or n: ")
			continue
		}
	}
	return true
}
