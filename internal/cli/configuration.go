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
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	CmdConfigurationDelete.Flags().Bool("unbind", false, "Unbind from applications before deleting")
	CmdConfiguration.AddCommand(CmdConfigurationShow)
	CmdConfiguration.AddCommand(CmdConfigurationCreate)
	CmdConfiguration.AddCommand(CmdConfigurationUpdate)
	CmdConfiguration.AddCommand(CmdConfigurationDelete)
	CmdConfiguration.AddCommand(CmdConfigurationBind)
	CmdConfiguration.AddCommand(CmdConfigurationUnbind)
	CmdConfiguration.AddCommand(CmdConfigurationList)

	CmdConfigurationList.Flags().Bool("all", false, "list all configurations")
	CmdConfigurationDelete.Flags().Bool("all", false, "delete all configurations")

	changeOptions(CmdConfigurationUpdate)

	CmdConfigurationCreate.Flags().StringSliceP("from-file", "f", []string{}, "values from files")
}

// CmdConfiguration implements the command: epinio configuration
var CmdConfiguration = &cobra.Command{
	Use:           "configuration",
	Aliases:       []string{"configurations"},
	Short:         "Epinio configuration features",
	Long:          `Handle configuration features with Epinio`,
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

// CmdConfigurationShow implements the command: epinio configuration show
var CmdConfigurationShow = &cobra.Command{
	Use:               "show NAME",
	Short:             "Configuration information",
	Long:              `Show detailed information of the named configuration.`,
	Args:              cobra.ExactArgs(1),
	RunE:              ConfigurationShow,
	ValidArgsFunction: matchingConfigurationFinder,
}

// CmdConfigurationCreate implements the command: epinio configuration create
var CmdConfigurationCreate = &cobra.Command{
	Use:   "create NAME (KEY VALUE)...",
	Short: "Create a configuration",
	Long:  `Create configuration by name and key/value dictionary.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("Not enough arguments, expected name")
		}
		if len(args)%2 == 0 {
			return errors.New("Last Key has no value")
		}
		return nil
	},
	RunE: ConfigurationCreate,
}

// CmdConfigurationUpdate implements the command: epinio configuration create
var CmdConfigurationUpdate = &cobra.Command{
	Use:               "update NAME [flags]",
	Short:             "Update a configuration",
	Long:              `Update configuration by name and change instructions through flags.`,
	Args:              cobra.ExactArgs(1),
	RunE:              ConfigurationUpdate,
	ValidArgsFunction: matchingConfigurationFinder,
}

// CmdConfigurationDelete implements the command: epinio configuration delete
var CmdConfigurationDelete = &cobra.Command{
	Use:   "delete NAME1 [NAME2 ...]",
	Short: "Delete one or more configurations",
	Long:  `Delete configurations by name.`,
	RunE:  ConfigurationDelete,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client.API.DisableVersionWarning()

		matches := filteredMatchingFinder(args, toComplete, client.ConfigurationMatching)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

// CmdConfigurationBind implements the command: epinio configuration bind
var CmdConfigurationBind = &cobra.Command{
	Use:               "bind NAME APP",
	Short:             "Bind a configuration to an application",
	Long:              `Bind configuration by name, to named application.`,
	Args:              cobra.ExactArgs(2),
	RunE:              ConfigurationBind,
	ValidArgsFunction: findConfigurationApp,
}

// CmdConfigurationUnbind implements the command: epinio configuration unbind
var CmdConfigurationUnbind = &cobra.Command{
	Use:               "unbind NAME APP",
	Short:             "Unbind configuration from an application",
	Long:              `Unbind configuration by name, from named application.`,
	Args:              cobra.ExactArgs(2),
	RunE:              ConfigurationUnbind,
	ValidArgsFunction: findConfigurationApp,
}

// CmdConfigurationList implements the command: epinio configuration list
var CmdConfigurationList = &cobra.Command{
	Use:   "list [--all]",
	Short: "Lists configurations",
	Long:  "Lists configurations in the targeted namespace, or all",
	RunE:  ConfigurationList,
}

// ConfigurationShow is the backend of command: epinio configuration show
func ConfigurationShow(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	err := client.ConfigurationDetails(args[0])
	if err != nil {
		return errors.Wrap(err, "error retrieving configuration")
	}

	return nil
}

// ConfigurationList is the backend of command: epinio configuration list
func ConfigurationList(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return errors.Wrap(err, "error reading option --all")
	}

	err = client.Configurations(all)
	if err != nil {
		return errors.Wrap(err, "error listing configurations")
	}

	return nil
}

// ConfigurationCreate is the backend of command: epinio configuration create
func ConfigurationCreate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	// Merge plain argument key/value data with k/v from options, i.e. files.
	kvAssigments := args[1:]
	kvFromFiles, err := cmd.Flags().GetStringSlice("from-file")
	if err != nil {
		return errors.Wrap(err, "failed to read option --from-file")
	}
	if len(kvFromFiles) > 0 {
		err, kvFiles := assignmentsFromFiles(kvFromFiles)
		if err != nil {
			return err
		}
		kvAssigments = append(kvAssigments, kvFiles...)
	}

	err = client.CreateConfiguration(args[0], kvAssigments)
	if err != nil {
		return errors.Wrap(err, "error creating configuration")
	}

	return nil
}

// ConfigurationUpdate is the backend of command: epinio configuration update
func ConfigurationUpdate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	// Process the --remove and --set options into operations (removals, assignments)

	removedKeys, err := cmd.Flags().GetStringSlice("remove")
	if err != nil {
		return errors.Wrap(err, "failed to read option --remove")
	}

	kvAssignments, err := cmd.Flags().GetStringSlice("set")
	if err != nil {
		return errors.Wrap(err, "failed to read option --set")
	}

	assignments := map[string]string{}
	for _, assignment := range kvAssignments {
		pieces := strings.Split(assignment, "=")
		if len(pieces) != 2 {
			return errors.New("Bad --set assignment `" + assignment + "`, expected `name=value` as value")
		}
		assignments[pieces[0]] = pieces[1]
	}

	err = client.UpdateConfiguration(args[0], removedKeys, assignments)
	if err != nil {
		return errors.Wrap(err, "error creating configuration")
	}

	return nil
}

// ConfigurationDelete is the backend of command: epinio configuration delete
func ConfigurationDelete(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	unbind, err := cmd.Flags().GetBool("unbind")
	if err != nil {
		return errors.Wrap(err, "error reading option --unbind")
	}

	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return errors.Wrap(err, "error reading option --all")
	}

	if all && len(args) > 0 {
		return errors.New("Conflict between --all and named configurations")
	}
	if !all && len(args) == 0 {
		return errors.New("No configurations specified for deletion")
	}

	err = client.DeleteConfiguration(args, unbind, all)
	if err != nil {
		return errors.Wrap(err, "error deleting configuration")
	}

	return nil
}

// ConfigurationBind is the backend of command: epinio configuration bind
func ConfigurationBind(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	err := client.BindConfiguration(args[0], args[1])
	if err != nil {
		return errors.Wrap(err, "error binding configuration")
	}

	return nil
}

// ConfigurationUnbind is the backend of command: epinio configuration unbind
func ConfigurationUnbind(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	err := client.UnbindConfiguration(args[0], args[1])
	if err != nil {
		return errors.Wrap(err, "error unbinding configuration")
	}

	return nil
}

// changeOptions initializes the --remove/-r and --set/-s options for
// the provided command.
func changeOptions(cmd *cobra.Command) {
	cmd.Flags().StringSliceP("set", "s", []string{}, "configuration key/value assignments to add/modify")
	cmd.Flags().StringSliceP("remove", "r", []string{}, "configuration keys to remove")

	// Note: No completion functionality. This would require asking the configuration for
	// its details so that the keys to remove can be matched. And add/modify cannot
	// check anyway.
}

func findConfigurationApp(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client.API.DisableVersionWarning()

	if len(args) == 1 {
		// #args == 1: app name.
		matches := client.AppsMatching(toComplete)
		return matches, cobra.ShellCompDirectiveNoFileComp
	}

	// #args == 0: configuration name.

	matches := client.ConfigurationMatching(toComplete)
	return matches, cobra.ShellCompDirectiveNoFileComp
}

func assignmentsFromFiles(fromFileSpecs []string) (error, []string) {
	results := []string{}
	for _, spec := range fromFileSpecs {
		var key string
		var valuefile string

		// The argument has two possible forms: `key=path`, or `path`.
		// The latter uses the filename part of the path as key.

		if strings.Contains(spec, "=") {
			pieces := strings.SplitN(spec, "=", 2)
			key = pieces[0]
			valuefile = pieces[1]
		} else {
			_, key = path.Split(spec)
			valuefile = spec
		}

		content, err := os.ReadFile(valuefile)
		if err != nil {
			return errors.Wrapf(err, "filesystem error"), nil
		}

		results = append(results, key, string(content))
	}

	return nil, results
}
