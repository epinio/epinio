package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const (
	Version = "0.1"
)

// Build time and commit information.
//
// ⚠️ WARNING: should only be set by "-ldflags".
var (
	BuildTime   string
	BuildCommit string
)

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd := &cobra.Command{
		Use:           "carrier",
		Short:         "Carrier cli",
		Long:          `carrier cli is the official command line interface for Carrier PaaS `,
		Version:       fmt.Sprintf("%s", Version),
		SilenceErrors: true,
	}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "install Carrier in your configured kubernetes cluster",
		Long:  `install Carrier PaaS in your configured kubernetes cluster`,
		Run:   install,
	}
	installCmd.Flags().BoolP("verbose", "v", true, "Wether to print logs to stdout")
	rootCmd.AddCommand(installCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
