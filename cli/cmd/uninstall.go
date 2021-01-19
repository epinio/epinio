// Package cmd includes all the subcommands supported by carrier cli
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/kubernetes"
)

// RegisterUninstall defines uninstall subcommand
// and adds it to the root command.
func RegisterUninstall(rootCmd *cobra.Command) {
	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "uninstall Carrier in your configured kubernetes cluster",
		Long:  `uninstall Carrier PaaS in your configured kubernetes cluster`,
		Run:   Uninstall,
	}

	rootCmd.AddCommand(uninstallCmd)
}

// Uninstall command uninstalls carrier on a configured cluster
func Uninstall(cmd *cobra.Command, args []string) {
	fmt.Println("Carrier uninstalling...")
	cluster, err := kubernetes.NewCluster(os.Getenv("KUBECONFIG"))
	ExitfIfError(err, "Couldn't get the cluster, check your config")

	err = installer.Uninstall(cluster)
	ExitfIfError(err, "Couldn't uninstall carrier")

	fmt.Println("Carrier uninstallation complete.")
}
