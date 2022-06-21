// Package cli contains all definitions pertaining to the user-visible
// commands of the epinio client. It provides the viper/cobra setup.
package cli

import (
	"fmt"
	"os"
	"runtime"

	"github.com/epinio/epinio/helpers/kubernetes/config"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	settings "github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/version"
	"github.com/go-logr/stdr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagSettingsFile string
)

// NewEpinioCLI returns the main `epinio` cli.
func NewEpinioCLI() *cobra.Command {
	return rootCmd
}

var rootCmd = &cobra.Command{
	Use:           "epinio",
	Short:         "Epinio cli",
	Long:          `epinio cli is the official command line interface for Epinio PaaS `,
	Version:       version.Version,
	SilenceErrors: true,
}

// Execute executes the root command.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	stdr.SetVerbosity(tracelog.TraceLevel())
	if err := rootCmd.Execute(); err != nil {
		termui.NewUI().Problem().Msg(err.Error())
		os.Exit(-1)
	}
}

func init() {
	pf := rootCmd.PersistentFlags()
	argToEnv := map[string]string{}

	settingsLocation := ""
	var err error
	if pf.Lookup("settings-file") == nil && os.Getenv("EPINIO_SETTINGS") == "" {
		settingsLocation, err = settings.DefaultLocation()
		if err != nil {
			// Error can happen on a read-only filesystem (like the one of the epinio
			// server Pod). User should set a location explicitly.
			errorMsg := fmt.Sprintf("A settings file wasn't set explicitly and the default location couldn't be used: %s", err.Error())
			panic(errorMsg)
		}
	}
	pf.StringVarP(&flagSettingsFile, "settings-file", "", settingsLocation, "set path of settings file")
	viper.BindPFlag("settings-file", pf.Lookup("settings-file"))
	argToEnv["settings-file"] = "EPINIO_SETTINGS"

	config.KubeConfigFlags(pf, argToEnv)
	tracelog.LoggerFlags(pf, argToEnv)
	duration.Flags(pf, argToEnv)

	pf.IntP("verbosity", "", 0, "Only print progress messages at or above this level (0 or 1, default 0)")
	viper.BindPFlag("verbosity", pf.Lookup("verbosity"))
	argToEnv["verbosity"] = "VERBOSITY"

	pf.BoolP("skip-ssl-verification", "", false, "Skip the verification of TLS certificates")
	viper.BindPFlag("skip-ssl-verification", pf.Lookup("skip-ssl-verification"))
	argToEnv["skip-ssl-verification"] = "SKIP_SSL_VERIFICATION"

	pf.BoolP("no-colors", "", false, "Suppress colorized output")
	viper.BindPFlag("no-colors", pf.Lookup("no-colors"))
	argToEnv["colors"] = "EPINIO_COLORS"

	config.AddEnvToUsage(rootCmd, argToEnv)

	rootCmd.AddCommand(CmdCompletion)
	rootCmd.AddCommand(CmdSettings)
	rootCmd.AddCommand(CmdInfo)
	rootCmd.AddCommand(CmdNamespace)
	rootCmd.AddCommand(CmdAppPush) // shorthand access to `app push`.
	rootCmd.AddCommand(CmdApp)
	rootCmd.AddCommand(CmdTarget)
	rootCmd.AddCommand(CmdConfiguration)
	rootCmd.AddCommand(CmdServer)
	rootCmd.AddCommand(cmdVersion)
	rootCmd.AddCommand(CmdServices)
	rootCmd.AddCommand(CmdLogin)

	// Hidden command providing developer tools
	rootCmd.AddCommand(CmdDebug)
}

var cmdVersion = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Epinio Version: %s\n", version.Version)
		fmt.Printf("Go Version: %s\n", runtime.Version())
	},
}
