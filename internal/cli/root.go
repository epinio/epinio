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
	kubeconfig     string
)

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	ExitfIfError(checkDependencies(), "Cannot operate")

	rootCmd := &cobra.Command{
		Use:           "epinio",
		Short:         "Epinio cli",
		Long:          `epinio cli is the official command line interface for Epinio PaaS `,
		Version:       fmt.Sprintf("%s", version.Version),
		SilenceErrors: true,
	}

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

	config.AddEnvToUsage(rootCmd, argToEnv)

	rootCmd.AddCommand(CmdCompletion)
	rootCmd.AddCommand(CmdInstall)
	rootCmd.AddCommand(CmdUninstall)
	rootCmd.AddCommand(CmdInfo)
	rootCmd.AddCommand(CmdOrg)
	rootCmd.AddCommand(CmdPush)
	rootCmd.AddCommand(CmdApp)
	rootCmd.AddCommand(CmdTarget)
	rootCmd.AddCommand(CmdEnable)
	rootCmd.AddCommand(CmdDisable)
	rootCmd.AddCommand(CmdService)
	rootCmd.AddCommand(CmdServer)
	rootCmd.AddCommand(cmdVersion)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
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

func checkDependencies() error {
	ok := true

	dependencies := []struct {
		CommandName string
	}{
		// update the 'prerequisites' of docs/install.md if you make changes here
		{CommandName: "kubectl"},
		{CommandName: "helm"},
		{CommandName: "sh"},
		{CommandName: "git"},
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
