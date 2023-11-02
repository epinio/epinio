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
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

//counterfeiter:generate -header ../../../LICENSE_HEADER . ConfigurationService
type ConfigurationService interface {
	Configurations(all bool) error
	ConfigurationDetails(configuration string) error
	CreateConfiguration(configuration string, kvAssigments []string) error
	DeleteConfiguration(configurations []string, unbind bool, all bool) error
	ConfigurationMatching(tocomplete string) []string
	UpdateConfiguration(configuration string, removedKeys []string, assignments map[string]string) error
	BindConfiguration(configuration, application string) error
	UnbindConfiguration(configuration, application string) error

	ConfigurationAppMatcher
}

// NewConfigurationCmd returns a new 'epinio configuration' command
func NewConfigurationCmd(client ConfigurationService, rootCfg *RootConfig) *cobra.Command {
	configurationCmd := &cobra.Command{
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

	configurationCmd.AddCommand(
		NewConfigurationBindCmd(client),
		NewConfigurationCreateCmd(client),
		NewConfigurationDeleteCmd(client),
		NewConfigurationListCmd(client, rootCfg),
		NewConfigurationShowCmd(client, rootCfg),
		NewConfigurationUnbindCmd(client),
		NewConfigurationUpdateCmd(client),
	)

	return configurationCmd
}

type ConfigurationListConfig struct {
	all bool
}

// NewConfigurationListCmd returns a new 'epinio configuration list' command
func NewConfigurationListCmd(client ConfigurationService, rootCfg *RootConfig) *cobra.Command {
	cfg := ConfigurationListConfig{}

	cmd := &cobra.Command{
		Use:   "list [--all]",
		Short: "Lists configurations",
		Long:  "Lists configurations in the targeted namespace, or all",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.Configurations(cfg.all)
			if err != nil {
				return errors.Wrap(err, "error listing configurations")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&cfg.all, "all", false, "list all configurations")
	cmd.Flags().VarP(rootCfg.Output, "output", "o", "sets output format [text|json]")
	bindFlag(cmd, "output")
	bindFlagCompletionFunc(cmd, "output", NewStaticFlagsCompletionFunc(rootCfg.Output.Allowed))

	return cmd
}

// NewConfigurationShowCmd returns a new 'epinio configuration show' command
func NewConfigurationShowCmd(client ConfigurationService, rootCfg *RootConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show NAME",
		Short: "Configuration information",
		Long:  `Show detailed information of the named configuration.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.ConfigurationDetails(args[0])
			if err != nil {
				return errors.Wrap(err, "error retrieving configuration")
			}

			return nil
		},
		ValidArgsFunction: FirstArgValidator(client.ConfigurationMatching),
	}

	cmd.Flags().VarP(rootCfg.Output, "output", "o", "sets output format [text|json]")
	bindFlag(cmd, "output")
	bindFlagCompletionFunc(cmd, "output", NewStaticFlagsCompletionFunc(rootCfg.Output.Allowed))

	return cmd
}

type ConfigurationCreateConfig struct {
	kvFromFiles []string
}

// NewConfigurationCreateCmd returns a new 'epinio configuration create' command
func NewConfigurationCreateCmd(client ConfigurationService) *cobra.Command {
	cfg := ConfigurationCreateConfig{}

	cmd := &cobra.Command{
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
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			// Merge plain argument key/value data with k/v from options, i.e. files.
			kvAssigments := args[1:]
			if len(cfg.kvFromFiles) > 0 {
				err, kvFiles := assignmentsFromFiles(cfg.kvFromFiles)
				if err != nil {
					return err
				}
				kvAssigments = append(kvAssigments, kvFiles...)
			}

			err := client.CreateConfiguration(args[0], kvAssigments)
			if err != nil {
				return errors.Wrap(err, "error creating configuration")
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&cfg.kvFromFiles, "from-file", "f", []string{}, "values from files")

	return cmd
}

type ConfigurationDeleteConfig struct {
	unbind bool
	all    bool
}

// NewConfigurationDeleteCmd returns a new 'epinio configuration delete' command
func NewConfigurationDeleteCmd(client ConfigurationService) *cobra.Command {
	cfg := ConfigurationDeleteConfig{}

	cmd := &cobra.Command{
		Use:   "delete NAME1 [NAME2 ...]",
		Short: "Delete one or more configurations",
		Long:  `Delete configurations by name.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			if cfg.all && len(args) > 0 {
				return errors.New("Conflict between --all and named configurations")
			}
			if !cfg.all && len(args) == 0 {
				return errors.New("No configurations specified for deletion")
			}

			err := client.DeleteConfiguration(args, cfg.unbind, cfg.all)
			if err != nil {
				return errors.Wrap(err, "error deleting configuration")
			}

			return nil
		},
		ValidArgsFunction: AnyArgsValidator(client.ConfigurationMatching),
	}

	cmd.Flags().BoolVar(&cfg.all, "all", false, "delete all configurations")
	cmd.Flags().BoolVar(&cfg.unbind, "unbind", false, "Unbind from applications before deleting")

	return cmd
}

// NewConfigurationUpdateCmd returns a new 'epinio configuration update' command
func NewConfigurationUpdateCmd(client ConfigurationService) *cobra.Command {
	cfg := ChangeConfig{}

	cmd := &cobra.Command{
		Use:   "update NAME [flags]",
		Short: "Update a configuration",
		Long:  `Update configuration by name and change instructions through flags.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			// Process --set information into a map
			assignments := map[string]string{}
			for _, assignment := range cfg.assigned {
				pieces := strings.Split(assignment, "=")
				if len(pieces) != 2 {
					return errors.New("Bad --set assignment `" + assignment + "`, expected `name=value` as value")
				}
				assignments[pieces[0]] = pieces[1]
			}

			err := client.UpdateConfiguration(args[0], cfg.removed, assignments)
			if err != nil {
				return errors.Wrap(err, "error creating configuration")
			}

			return nil
		},
		ValidArgsFunction: FirstArgValidator(client.ConfigurationMatching),
	}

	changeOptions(cmd, &cfg)

	return cmd
}

// NewConfigurationBindCmd returns a new 'epinio configuration bind' command
func NewConfigurationBindCmd(client ConfigurationService) *cobra.Command {
	return &cobra.Command{
		Use:   "bind NAME APP",
		Short: "Bind a configuration to an application",
		Long:  `Bind configuration by name, to named application.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.BindConfiguration(args[0], args[1])
			if err != nil {
				return errors.Wrap(err, "error binding configuration")
			}

			return nil
		},
		ValidArgsFunction: NewConfigurationAppMatcherFunc(client),
	}
}

// NewConfigurationUnbindCmd returns a new 'epinio configuration unbind' command
func NewConfigurationUnbindCmd(client ConfigurationService) *cobra.Command {
	return &cobra.Command{
		Use:   "unbind NAME APP",
		Short: "Unbind configuration from an application",
		Long:  `Unbind configuration by name, from named application.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.UnbindConfiguration(args[0], args[1])
			if err != nil {
				return errors.Wrap(err, "error unbinding configuration")
			}

			return nil
		},
		ValidArgsFunction: NewConfigurationAppMatcherFunc(client),
	}
}

// / / // /// ///// //////// /////////////

type ChangeConfig struct {
	removed  []string
	assigned []string
}

// changeOptions initializes the --unset/-u and --set/-s options for the provided command.
// It also initializes the old --remove/-r options, and marks them as deprecated.
func changeOptions(cmd *cobra.Command, cfg *ChangeConfig) {
	cmd.Flags().StringSliceVarP(&cfg.assigned, "set", "s", []string{}, "configuration key/value assignments to add/modify")
	cmd.Flags().StringSliceVarP(&cfg.removed, "unset", "u", []string{}, "configuration keys to remove")
	cmd.Flags().StringSliceVarP(&cfg.removed, "remove", "r", []string{}, "(deprecated) configuration keys to remove")
	err := cmd.Flags().MarkDeprecated("remove", "please use --unset instead")
	if err != nil {
		log.Fatal(err)
	}

	// Note: No completion functionality. This would require asking the configuration for
	// its details so that the keys to remove can be matched. And add/modify cannot
	// check anyway.
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
