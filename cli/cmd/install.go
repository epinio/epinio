// Package cmd includes all the subcommands supported by carrier cli
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/deployments"
	"github.com/suse/carrier/cli/kubernetes"
)

var installer = kubernetes.Installer{
	Deployments: []kubernetes.Deployment{
		&deployments.Traefik{},
		&deployments.Quarks{},
		&deployments.Gitea{},
	},
}

func RegisterInstall(rootCmd *cobra.Command) {
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "install Carrier in your configured kubernetes cluster",
		Long:  `install Carrier PaaS in your configured kubernetes cluster`,
		Run:   Install,
	}
	installCmd.Flags().BoolP("verbose", "v", true, "Wether to print logs to stdout")
	installCmd.Flags().BoolP("non-interactive", "n", false, "Whether to ask the user or not")

	installer.GatherNeededOptions()
	for _, opt := range installer.NeededOptions {
		// Translate option name
		flagName := strings.ReplaceAll(opt.Name, "_", "-")

		// Declare option's flag, type-dependent
		switch opt.Type {
		case kubernetes.BooleanType:
			installCmd.Flags().Bool(flagName, opt.Default.(bool), opt.Description)
		case kubernetes.StringType:
			installCmd.Flags().String(flagName, opt.Default.(string), opt.Description)
		case kubernetes.IntType:
			installCmd.Flags().Int(flagName, opt.Default.(int), opt.Description)
		}
	}

	rootCmd.AddCommand(installCmd)
}

// Install command installs carrier on a configured cluster
func Install(cmd *cobra.Command, args []string) {
	fmt.Println("Carrier installing...")

	cluster, err := kubernetes.NewCluster(os.Getenv("KUBECONFIG"))
	ExitfIfError(err, "Couldn't get the cluster, check your config")

	// Hack? Override static default for system domain with a
	// function which queries the cluster for the necessary
	// data. If that data could not be found the system will fall
	// back to cli option and/or interactive entry by the user.
	//
	// NOTE: This is function is set here and not in the gitea
	// definition because the function has to have access to the
	// cluster in question, and that information is only available
	// now, not at deployment declaration time.

	domain, err := installer.NeededOptions.GetOpt("system_domain", "")
	if err != nil {
		ExitfIfError(err, "Couldn't install carrier")
	}
	domain.DynDefaultFunc = func(o *kubernetes.InstallationOption) error {
		ips := cluster.GetPlatform().ExternalIPs()
		if len(ips) > 0 {
			domain := fmt.Sprintf("%s.nip.io", ips[0])
			o.Value = domain
		}
		// else leave invalid, to be handled by cli option
		// reader or interactive entry
		return nil
	}

	installer.PopulateNeededOptions(kubernetes.NewCLIOptionsReader(cmd))

	nonInteractive, err := cmd.Flags().GetBool("non-interactive")
	ExitfIfError(err, "Couldn't install carrier")

	if nonInteractive {
		installer.PopulateNeededOptions(kubernetes.NewDefaultOptionsReader())
	} else {
		installer.PopulateNeededOptions(kubernetes.NewInteractiveOptionsReader(os.Stdout, os.Stdin))
	}

	installer.ShowNeededOptions()

	// TODO (post MVP): Run a validation phase which perform
	// additional checks on the values. For example range limits,
	// proper syntax of the string, etc. do it as pghase, and late
	// to report all problems at once, instead of early and
	// piecemal.

	err = installer.Install(cluster)
	ExitfIfError(err, "Couldn't install carrier")

	fmt.Println("Carrier installation complete.")
}
