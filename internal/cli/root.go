// Package cli contains all definitions pertaining to the user-visible
// commands of the epinio client. It provides the viper/cobra setup.
package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/epinio/epinio/helpers/kubernetes/config"
	"github.com/epinio/epinio/helpers/tracelog"
	pconfig "github.com/epinio/epinio/internal/cli/config"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/version"
	"github.com/kyokomi/emoji"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagConfigFile string
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
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	pf := rootCmd.PersistentFlags()
	argToEnv := map[string]string{}

	pf.StringVarP(&flagConfigFile, "config-file", "", pconfig.DefaultLocation(),
		"set path of configuration file")
	viper.BindPFlag("config-file", pf.Lookup("config-file"))
	argToEnv["config-file"] = "EPINIO_CONFIG"

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

	rootCmd.AddCommand(cmdVersion)
}

var cmdVersion = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Epinio Version: %s\n", version.Version)
		fmt.Printf("Go Version: %s\n", runtime.Version())
	},
}

// checkDependencies is a helper which checks the client's environment
// for the presence of a number of required supporting commands.
func checkDependencies() error {
	ok := true

	dependencies := []struct {
		CommandName string
	}{
		{CommandName: "kubectl"},
		{CommandName: "helm"},
	}

	for _, dependency := range dependencies {
		_, err := exec.LookPath(dependency.CommandName)
		if err != nil {
			fmt.Println(emoji.Sprintf(":fire:Not found: %s", dependency.CommandName))
			ok = false
		}
	}

	if ok {
		return nil
	}

	return errors.New("Please check your PATH, some of our dependencies were not found")
}
