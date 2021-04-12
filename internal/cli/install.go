package cli

import (
	"context"
	"sync"

	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/kubernetes"
	"github.com/epinio/epinio/termui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var NeededOptions = kubernetes.InstallationOptions{
	{
		Name:        "system_domain",
		Description: "The domain you are planning to use for Epinio. Should be pointing to the traefik public IP (Leave empty to use a omg.howdoi.website domain).",
		Type:        kubernetes.StringType,
		Default:     "",
		Value:       "",
	},
	{
		Name:        "email_address",
		Description: "The email address you are planning to use for getting notifications about your certificates",
		Type:        kubernetes.StringType,
		Default:     "epinio@suse.com",
		Value:       "",
	},
}

const (
	DefaultOrganization = "workspace"
)

var CmdInstall = &cobra.Command{
	Use:           "install",
	Short:         "install Epinio in your configured kubernetes cluster",
	Long:          `install Epinio PaaS in your configured kubernetes cluster`,
	Args:          cobra.ExactArgs(0),
	RunE:          Install,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	CmdInstall.Flags().BoolP("interactive", "i", false, "Whether to ask the user or not (default not)")

	NeededOptions.AsCobraFlagsFor(CmdInstall)
}

// Install command installs epinio on a configured cluster
func Install(cmd *cobra.Command, args []string) error {
	installClient, installCleanup, err := clients.NewInstallClient(cmd.Flags(), &NeededOptions)
	defer func() {
		if installCleanup != nil {
			installCleanup()
		}
	}()

	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = installClient.Install(cmd)
	if err != nil {
		return errors.Wrap(err, "error installing Epinio")
	}

	// Installation complete. Run `org create`, and `target`.

	// TODO: Target remote epinio server instead of starting one
	port := viper.GetInt("port")
	httpServerWg := &sync.WaitGroup{}
	httpServerWg.Add(1)
	ui := termui.NewUI()
	srv, listeningPort, err := startEpinioServer(httpServerWg, port, ui)
	if err != nil {
		return err
	}

	// TODO: NOTE: This is a hack until the server is running inside the cluster
	cmd.Flags().String("server-url", "http://127.0.0.1:"+listeningPort, "")

	epinio_client, err := clients.NewEpinioClient(cmd.Flags())
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	// Post Installation Tasks:
	// - Create and target a default organization, so that the
	//   user can immediately begin to push applications.
	//
	// Dev Note: The targeting is done to ensure that a epinio
	// config left over from a previous installation will contain
	// a valid organization. Without it may contain the name of a
	// now invalid organization from said previous install. This
	// then breaks push and other commands in non-obvious ways.

	err = epinio_client.CreateOrg(DefaultOrganization)
	if err != nil {
		return errors.Wrap(err, "error creating org")
	}

	if err := srv.Shutdown(context.Background()); err != nil {
		return err
	}
	httpServerWg.Wait()

	err = epinio_client.Target(DefaultOrganization)
	if err != nil {
		return errors.Wrap(err, "failed to set target")
	}

	return nil
}
