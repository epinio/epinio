package cli

import (
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

// CmdApp implements the epinio -app command
var CmdApp = &cobra.Command{
	Use:           "app",
	Aliases:       []string{"apps"},
	Short:         "Epinio application features",
	Long:          `Manage epinio application`,
	Args:          cobra.ExactArgs(0),
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	CmdApp.AddCommand(CmdAppShow)
	CmdApp.AddCommand(CmdAppList)
	CmdApp.AddCommand(CmdDeleteApp)
	CmdApp.AddCommand(CmdPush)
	CmdApp.AddCommand(CmdAppUpdate)
}

// CmdAppList implements the epinio `apps list` command
var CmdAppList = &cobra.Command{
	Use:   "list",
	Short: "Lists all applications",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clients.NewEpinioClient(cmd.Flags())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.Apps()
		if err != nil {
			return errors.Wrap(err, "error listing apps")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

// CmdAppShow implements the epinio `apps show` command
var CmdAppShow = &cobra.Command{
	Use:   "show NAME",
	Short: "Describe the named application",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clients.NewEpinioClient(cmd.Flags())

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.AppShow(args[0])
		if err != nil {
			return errors.Wrap(err, "error listing apps")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := clients.NewEpinioClient(cmd.Flags())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := app.AppsMatching(toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

// CmdAppUpdate is used by the epinio `apps update` command to scale
// a single app
var CmdAppUpdate = &cobra.Command{
	Use:   "update NAME",
	Short: "Update the named application",
	Long:  "Update the running application's attributes (e.g. instances)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clients.NewEpinioClient(cmd.Flags())

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		i, err := instances(cmd)
		if err != nil {
			return err
		}
		if i == nil {
			d := v1.DefaultInstances
			i = &d
		}
		err = client.AppUpdate(args[0], *i)
		if err != nil {
			return errors.Wrap(err, "error updating the app")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := clients.NewEpinioClient(cmd.Flags())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := app.AppsMatching(toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	flags := CmdAppUpdate.Flags()
	flags.Int32P("instances", "i", 1, "The number of instances the application should have")
	cobra.MarkFlagRequired(flags, "instances")
}
