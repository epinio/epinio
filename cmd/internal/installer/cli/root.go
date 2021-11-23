// Package cli contains the dev/CI installer's CLI
package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/epinio/epinio/helpers/kubernetes/config"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/version"
	"github.com/kyokomi/emoji"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:           "epinio",
	Short:         "Epinio Installer",
	Long:          `epinio installer is the command line interface for CI/dev installs of the Epinio PaaS`,
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

	pf.StringP("manifest", "m", "epinio-install.yml", "set path of configuration file")
	viper.BindPFlag("manifest", pf.Lookup("manifest"))
	argToEnv["manifest"] = "EPINIO_MANIFEST"

	config.KubeConfigFlags(pf, argToEnv)
	tracelog.LoggerFlags(pf, argToEnv)
	duration.Flags(pf, argToEnv)

	pf.BoolP("no-colors", "", false, "Suppress colorized output")
	viper.BindPFlag("no-colors", pf.Lookup("no-colors"))
	argToEnv["colors"] = "EPINIO_COLORS"

	config.AddEnvToUsage(rootCmd, argToEnv)

	rootCmd.AddCommand(CmdInstall)
	rootCmd.AddCommand(CmdUninstall)
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

	return errors.New("please check your PATH, some of our dependencies were not found")
}

// exitfIfError stops the application with an error exit code, after
// printing error and message, if there is an error.
func exitfIfError(err error, message string) {
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("%s: %w", message, err))
		os.Exit(1)
	}
}
