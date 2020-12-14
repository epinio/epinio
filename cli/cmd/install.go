// Package cmd includes all the subcommands supported by carrier cli
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/kubernetes"
)

// install command installs carrier on a configured cluster
func install(cmd *cobra.Command, args []string) {
	fmt.Println("Carrier installing...")

	cluster, err := kubernetes.NewCluster(kubeconfig)
	ExitfIfError(err, "Something went wrong")

	// WIP WIP WIP: collect needed input from deployments and present all questions
	// to the user in the begining. Don't wait until it's needed.
	neededInput := []kubernetes.UserInput{}
	for _, deployment := range []kubernetes.Deployment{Kpack{}, Gitea{}} {
		deployment.CollectInput()
	}

	deployment.Install(cluster)
}
