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
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

//counterfeiter:generate -header ../../../LICENSE_HEADER . NamespaceService
type NamespaceService interface {
	CreateNamespace(namespace string) error
	Namespaces() error
	DeleteNamespace(namespaces []string, force, all bool) error
	ShowNamespace(namespace string) error
	NamespacesMatching(toComplete string) []string
}

// NewNamespaceCmd returns a new 'epinio namespace' command
func NewNamespaceCmd(client NamespaceService, rootCfg *RootConfig) *cobra.Command {
	namespaceCmd := &cobra.Command{
		Use:           "namespace",
		Aliases:       []string{"namespaces"},
		Short:         "Epinio-controlled namespaces",
		Long:          `Manage epinio-controlled namespaces`,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.MinimumNArgs(1),
	}

	namespaceCmd.AddCommand(
		NewNamespaceCreateCmd(client),
		NewNamespaceListCmd(client, rootCfg),
		NewNamespaceDeleteCmd(client),
		NewNamespaceShowCmd(client, rootCfg),
	)

	return namespaceCmd
}

// NewNamespaceCreateCmd returns a new 'epinio namespace create' command
func NewNamespaceCreateCmd(client NamespaceService) *cobra.Command {
	return &cobra.Command{
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
}

// NewNamespaceListCmd returns a new 'epinio namespace list' command
func NewNamespaceListCmd(client NamespaceService, rootCfg *RootConfig) *cobra.Command {
	namespaceListCmd := &cobra.Command{
		Use:   "list",
		Short: "Lists all epinio-controlled namespaces",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.Namespaces()
			if err != nil {
				return errors.Wrap(err, "error listing epinio-controlled namespaces")
			}

			return nil
		},
	}

	namespaceListCmd.Flags().VarP(rootCfg.Output, "output", "o", "sets output format [text|json]")
	bindFlag(namespaceListCmd, "output")
	bindFlagCompletionFunc(namespaceListCmd, "output", NewStaticFlagsCompletionFunc(rootCfg.Output.Allowed))

	return namespaceListCmd
}

type NamespaceDeleteConfig struct {
	force bool
	all   bool
}

// NewNamespaceDeleteCmd returns a new 'epinio namespace delete' command
func NewNamespaceDeleteCmd(client NamespaceService) *cobra.Command {
	cfg := NamespaceDeleteConfig{}

	namespaceDeleteCmd := &cobra.Command{
		Use:               "delete NAME",
		Short:             "Deletes an epinio-controlled namespace",
		ValidArgsFunction: FirstArgValidator(client.NamespacesMatching),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.DeleteNamespace(args, cfg.force, cfg.all)
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

	namespaceDeleteCmd.Flags().BoolVarP(&cfg.force, "force", "f", false, "force namespace deletion")
	namespaceDeleteCmd.Flags().BoolVar(&cfg.all, "all", false, "delete all namespaces")

	return namespaceDeleteCmd
}

// NewNamespaceShowCmd returns a new 'epinio namespace show' command
func NewNamespaceShowCmd(client NamespaceService, rootCfg *RootConfig) *cobra.Command {
	namespaceShowCmd := &cobra.Command{
		Use:               "show NAME",
		Short:             "Shows the details of an epinio-controlled namespace",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: FirstArgValidator(client.NamespacesMatching),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.ShowNamespace(args[0])
			if err != nil {
				return errors.Wrap(err, "error showing epinio-controlled namespace")
			}

			return nil
		},
	}

	namespaceShowCmd.Flags().VarP(rootCfg.Output, "output", "o", "sets output format [text|json]")
	bindFlag(namespaceShowCmd, "output")
	bindFlagCompletionFunc(namespaceShowCmd, "output", NewStaticFlagsCompletionFunc(rootCfg.Output.Allowed))

	return namespaceShowCmd
}
