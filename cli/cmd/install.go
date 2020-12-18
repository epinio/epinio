// Package cmd includes all the subcommands supported by carrier cli
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/deployments"
	"github.com/suse/carrier/cli/kubernetes"
)

// Install command installs carrier on a configured cluster
func Install(cmd *cobra.Command, args []string) {
	fmt.Println("Carrier installing...")
	installer := kubernetes.Installer{
		Deployments: []kubernetes.Deployment{
			&deployments.Traefik{},
			&deployments.Quarks{},
		},
	}
	installer.GatherNeededOptions()
	installer.PopulateNeededOptions(nil)
	cluster, err := kubernetes.NewCluster(os.Getenv("KUBECONFIG"))
	ExitfIfError(err, "Couldn't get the cluster, check your config")
	err = installer.Install(cluster)
	ExitfIfError(err, "Couldn't install carrier")

	fmt.Println("Carrier installation complete.")
}
