// Package cmd includes all the subcommands supported by carrier cli
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/kubernetes"
)

// install command installs carrier on a configured cluster
func Install(cmd *cobra.Command, args []string) {
	fmt.Println("Carrier installing...")
	// TODO: Actually install some deployment
	installer := kubernetes.Installer{}
	installer.GatherNeededOptions()
	installer.PopulateNeededOptions(nil)
	cluster, err := kubernetes.NewCluster("") // TODO: find kubeconfig
	ExitfIfError(err, "Couldn't get the cluster, check your config")
	installer.Install(cluster)
}
