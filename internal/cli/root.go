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
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/epinio/epinio/helpers/kubernetes/config"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	settings "github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/version"
	"github.com/go-logr/stdr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagSettingsFile string
)

// NewRootCmd returns the rootCmd, that is the main `epinio` cli.
func NewRootCmd() (*cobra.Command, error) {
	rootCmd := &cobra.Command{
		Args:          cobra.MaximumNArgs(0),
		Use:           "epinio",
		Short:         "Epinio cli",
		Long:          `epinio cli is the official command line interface for Epinio PaaS `,
		Version:       version.Version,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			stdr.SetVerbosity(tracelog.TraceLevel())
		},
	}

	pf := rootCmd.PersistentFlags()
	argToEnv := map[string]string{}

	settingsLocation := ""
	var err error
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

	pf.IntP("verbosity", "", 0, "Only print progress messages at or above this level (0 or 1, default 0)")
	if err = viper.BindPFlag("verbosity", pf.Lookup("verbosity")); err != nil {
		return nil, err
	}
	argToEnv["verbosity"] = "VERBOSITY"

	pf.BoolP("skip-ssl-verification", "", false, "Skip the verification of TLS certificates")
	if err = viper.BindPFlag("skip-ssl-verification", pf.Lookup("skip-ssl-verification")); err != nil {
		return nil, err
	}
	argToEnv["skip-ssl-verification"] = "SKIP_SSL_VERIFICATION"

	pf.BoolP("no-colors", "", false, "Suppress colorized output")
	if err = viper.BindPFlag("no-colors", pf.Lookup("no-colors")); err != nil {
		return nil, err
	}

	// Environment variable EPINIO_COLORS is handled in settings/settings.go,
	// as part of handling the settings file.

	config.AddEnvToUsage(rootCmd, argToEnv)

	client, err := usercmd.New(context.Background())
	if err != nil {
		return nil, errors.New("error initializing cli")
	}

	rootCmd.AddCommand(
		CmdCompletion,
		CmdSettings,
		NewInfoCmd(client),
		NewClientSyncCmd(client),
		CmdNamespace,
		CmdAppPush, // shorthand access to `app push`
		CmdApp,
		CmdTarget,
		CmdConfiguration,
		CmdServer,
		cmdVersion,
		CmdServices,
		CmdLogin,
		CmdLogout,
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

var cmdVersion = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Epinio Version: %s\n", version.Version)
		fmt.Printf("Go Version: %s\n", runtime.Version())
	},
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
