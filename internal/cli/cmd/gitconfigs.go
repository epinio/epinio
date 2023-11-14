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
	"strings"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// NewGitconfigCmd returns a new 'epinio gitconfig' command
func NewGitconfigCmd(client *usercmd.EpinioClient) *cobra.Command {
	gitConfigCmd := &cobra.Command{
		Use:           "gitconfig",
		Aliases:       []string{"gitconfigs"},
		Short:         "Epinio git configuration management",
		Long:          `Manage git configurations`,
		SilenceErrors: true,
		Args:          cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Usage(); err != nil {
				return err
			}
			return fmt.Errorf(`Unknown method "%s"`, args[0])
		},
	}

	gitConfigCmd.AddCommand(
		NewGitconfigListCmd(client),
		NewGitconfigCreateCmd(client),
		NewGitconfigDeleteCmd(client),
		NewGitconfigShowCmd(client),
	)

	return gitConfigCmd
}

// NewGitconfigListCmd returns a new 'epinio gitconfig list' command
func NewGitconfigListCmd(client *usercmd.EpinioClient) *cobra.Command {
	return &cobra.Command{
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
}

type GitconfigCreateConfig struct {
	skipSSL     bool
	username    string
	password    string
	userOrg     string
	gitProvider string
	repository  string
	certFile    string
}

// NewGitconfigCreateCmd returns a new 'epinio gitconfig create' command
func NewGitconfigCreateCmd(client *usercmd.EpinioClient) *cobra.Command {
	cfg := GitconfigCreateConfig{}

	gitconfigCreateCmd := &cobra.Command{
		Use:   "create ID URL [flags]",
		Short: "Creates a git configuration",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			id := args[0]
			url := args[1]

			err := client.CreateGitconfig(
				id, cfg.gitProvider, url,
				cfg.username, cfg.password, cfg.userOrg,
				cfg.repository, cfg.certFile, cfg.skipSSL,
			)
			if err != nil {
				return errors.Wrap(err, "error creating git configuration")
			}

			return nil
		},
	}

	gitconfigCreateCmd.Flags().BoolVar(&cfg.skipSSL, "skip-ssl", false, "skip SSL")
	gitconfigCreateCmd.Flags().StringVar(&cfg.username, "username", "", "user name for logging into the host")
	gitconfigCreateCmd.Flags().StringVar(&cfg.password, "password", "", "password for logging into the host")
	gitconfigCreateCmd.Flags().StringVar(&cfg.userOrg, "user-org", "", "user/org holding repository")
	gitconfigCreateCmd.Flags().StringVar(&cfg.repository, "repository", "", "specific repository")
	gitconfigCreateCmd.Flags().StringVar(&cfg.certFile, "cert-file", "", "path to file holding supporting certificates")

	gitconfigCreateCmd.Flags().StringVar(&cfg.gitProvider, "git-provider", "git", "Git provider code [git|github|github_enterprise|gitlab|gitlab_enterprise]")
	bindFlagCompletionFunc(gitconfigCreateCmd, "git-provider", NewStaticFlagsCompletionFunc(models.ValidProviders))

	return gitconfigCreateCmd
}

type GitconfigDeleteConfig struct {
	all bool
}

// NewGitconfigDeleteCmd returns a new 'epinio gitconfig delete' command
func NewGitconfigDeleteCmd(client *usercmd.EpinioClient) *cobra.Command {
	cfg := &GitconfigDeleteConfig{}

	gitconfigDeleteCmd := &cobra.Command{
		Use:               "delete NAME",
		Short:             "Delete git configurations",
		ValidArgsFunction: NewGitconfigMatcherFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.DeleteGitconfig(args, cfg.all)
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

	gitconfigDeleteCmd.Flags().BoolVar(&cfg.all, "all", false, "delete all git configurations")

	return gitconfigDeleteCmd
}

// NewGitconfigShowCmd returns a new 'epinio gitconfig delete' command
func NewGitconfigShowCmd(client *usercmd.EpinioClient) *cobra.Command {
	return &cobra.Command{
		Use:               "show NAME",
		Short:             "Shows the details of a git configuration",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewGitconfigMatcherFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.ShowGitconfig(args[0])
			if err != nil {
				return errors.Wrap(err, "error showing git configuration")
			}

			return nil
		},
	}
}
