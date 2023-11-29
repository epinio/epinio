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

// Package cli contains all definitions pertaining to the user-visible
// commands of the epinio client. It provides the viper/cobra setup.
package cli

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes/config"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/cmd"
	settings "github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/cli/termui"
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/version"
	"github.com/go-logr/stdr"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	client *usercmd.EpinioClient

	flagSettingsFile string
	flagHeaders      []string
)

// NewRootCmd returns the rootCmd, that is the main `epinio` cli.
func NewRootCmd() (*cobra.Command, error) {
	cfg := cmd.NewRootConfig()

	var err error
	client, err = usercmd.New()
	if err != nil {
		return nil, errors.Wrap(err, "initializing cli")
	}

	rootCmd := &cobra.Command{
		Args:          cobra.MaximumNArgs(0),
		Use:           "epinio",
		Short:         "Epinio cli",
		Long:          `epinio cli is the official command line interface for Epinio PaaS `,
		Version:       version.Version,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			stdr.SetVerbosity(tracelog.TraceLevel())

			err := client.Init(cmd.Context())
			if err != nil {
				return errors.Wrap(err, "initializing client")
			}

			if cfg.Output.String() == "json" {
				client.UI().EnableJSON()
				client.API.DisableVersionWarning()
			}

			for _, header := range flagHeaders {
				headerKeyValue := strings.SplitN(header, ":", 2)

				// empty headers are valid
				var headerValue string
				if len(headerKeyValue) > 1 {
					headerValue = headerKeyValue[1]
				}
				client.API.SetHeader(headerKeyValue[0], strings.TrimSpace(headerValue))
			}
			return nil
		},
	}

	pf := rootCmd.PersistentFlags()
	argToEnv := map[string]string{}

	settingsLocation := ""

	if pf.Lookup("settings-file") == nil && os.Getenv("EPINIO_SETTINGS") == "" {
		settingsLocation, err = settings.DefaultLocation()
		if err != nil {
			// Error can happen on a read-only filesystem (like the one of the epinio
			// server Pod). User should set a location explicitly.
			return nil, fmt.Errorf("A settings file wasn't set explicitly and the default location couldn't be used: %s", err.Error())
		}
	}

	pf.StringVarP(&flagSettingsFile, "settings-file", "", settingsLocation, "set path of settings file")
	if err = viper.BindPFlag("settings-file", pf.Lookup("settings-file")); err != nil {
		return nil, err
	}
	argToEnv["settings-file"] = "EPINIO_SETTINGS"

	config.KubeConfigFlags(pf, argToEnv)
	tracelog.LoggerFlags(pf, argToEnv)
	duration.Flags(pf, argToEnv)

	pf.Int("verbosity", 0, "Only print progress messages at or above this level (0 or 1, default 0)")
	if err = viper.BindPFlag("verbosity", pf.Lookup("verbosity")); err != nil {
		return nil, err
	}
	argToEnv["verbosity"] = "VERBOSITY"

	pf.Bool("skip-ssl-verification", false, "Skip the verification of TLS certificates")
	if err = viper.BindPFlag("skip-ssl-verification", pf.Lookup("skip-ssl-verification")); err != nil {
		return nil, err
	}
	argToEnv["skip-ssl-verification"] = "SKIP_SSL_VERIFICATION"

	pf.StringArrayVarP(&flagHeaders, "header", "H", []string{}, "Add custom header to every request executed")
	if err = viper.BindPFlag("header", pf.Lookup("header")); err != nil {
		return nil, err
	}

	pf.Bool("no-colors", false, "Suppress colorized output")
	if err = viper.BindPFlag("no-colors", pf.Lookup("no-colors")); err != nil {
		return nil, err
	}

	// Environment variable EPINIO_COLORS is handled in settings/settings.go,
	// as part of handling the settings file.

	config.AddEnvToUsage(rootCmd, argToEnv)

	rootCmd.AddCommand(
		CmdCompletion,
		cmd.NewSettingsCmd(client),
		cmd.NewInfoCmd(client, cfg),
		cmd.NewClientSyncCmd(client),
		cmd.NewGitconfigCmd(client),
		cmd.NewNamespaceCmd(client, cfg),
		cmd.NewAppPushCmd(client), // shorthand access to `app push`
		cmd.NewApplicationsCmd(client, cfg),
		cmd.NewTargetCmd(client),
		cmd.NewConfigurationCmd(client, cfg),
		CmdServer,
		cmd.NewVersionCmd(),
		cmd.NewServicesCmd(client, cfg),
		cmd.NewLoginCmd(client),
		cmd.NewLogoutCmd(client),
		cmd.NewExportRegistriesCmd(client),
	)

	// Hidden command providing developer tools
	rootCmd.AddCommand(CmdDebug)

	return rootCmd, nil
}

// Execute executes the root command.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd, err := NewRootCmd()
	if err != nil {
		termui.NewUI().Problem().Msg(err.Error())
		os.Exit(-1)
	}

	if err := rootCmd.Execute(); err != nil {
		termui.NewUI().Problem().Msg(err.Error())
		os.Exit(-1)
	}
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
