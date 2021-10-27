package cli

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/internal/manifest"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

// CmdApp implements the  command: epinio app
var CmdApp = &cobra.Command{
	Use:           "app",
	Aliases:       []string{"apps"},
	Short:         "Epinio application features",
	Long:          `Manage epinio application`,
	SilenceErrors: true,
	SilenceUsage:  true,
	Args:          cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cmd.Usage(); err != nil {
			return err
		}
		return fmt.Errorf(`Unknown method "%s"`, args[0])
	},
}

func init() {
	flags := CmdAppLogs.Flags()
	flags.Bool("follow", false, "follow the logs of the application")
	flags.Bool("staging", false, "show the staging logs of the application")

	CmdAppCreate.Flags().StringSliceP("domain", "d", []string{}, "Custom domain to use as the application's route (a subdomain of the default domain will be used if this is not set). Can be set multiple times to use multiple domains with the same application.")
	CmdAppUpdate.Flags().StringSliceP("domain", "d", []string{}, "Custom domain to use as the application's route (a subdomain of the default domain will be used if this is not set). Can be set multiple times to use multiple domains with the same application.")

	bindOption(CmdAppCreate)
	bindOption(CmdAppUpdate)
	envOption(CmdAppCreate)
	envOption(CmdAppUpdate)
	instancesOption(CmdAppCreate)
	instancesOption(CmdAppUpdate)

	flags = CmdAppList.Flags()
	flags.Bool("all", false, "list all applications")

	CmdApp.AddCommand(CmdAppCreate)
	CmdApp.AddCommand(CmdAppEnv) // See env.go for implementation
	CmdApp.AddCommand(CmdAppList)
	CmdApp.AddCommand(CmdAppLogs)
	CmdApp.AddCommand(CmdAppManifest)
	CmdApp.AddCommand(CmdAppShow)
	CmdApp.AddCommand(CmdAppUpdate)
	CmdApp.AddCommand(CmdAppDelete)
	CmdApp.AddCommand(CmdAppPush) // See push.go for implementation
}

// CmdAppList implements the command: epinio app list
var CmdAppList = &cobra.Command{
	Use:   "list [--all]",
	Short: "Lists applications",
	Long:  "Lists applications in the targeted namespace, or all",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return errors.Wrap(err, "error reading option --all")
		}

		err = client.Apps(all)
		if err != nil {
			return errors.Wrap(err, "error listing apps")
		}

		return nil
	},
}

// CmdAppCreate implements the command: epinio apps create
var CmdAppCreate = &cobra.Command{
	Use:   "create NAME",
	Short: "Create just the app, without creating a workload",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		m, err := manifest.UpdateISE(models.ApplicationManifest{}, cmd)
		if err != nil {
			return errors.Wrap(err, "unable to get app configuration")
		}

		m, err = manifest.UpdateDomains(m, cmd)
		if err != nil {
			return err
		}

		err = client.AppCreate(args[0], m.Configuration)
		if err != nil {
			return errors.Wrap(err, "error creating app")
		}

		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := usercmd.New()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := app.AppsMatching(context.Background(), toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

// CmdAppShow implements the command: epinio apps show
var CmdAppShow = &cobra.Command{
	Use:   "show NAME",
	Short: "Describe the named application",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.AppShow(args[0])
		if err != nil {
			return errors.Wrap(err, "error showing app")
		}

		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := usercmd.New()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := app.AppsMatching(context.Background(), toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

// CmdAppLogs implements the command: epinio apps logs
var CmdAppLogs = &cobra.Command{
	Use:   "logs NAME",
	Short: "Streams the logs of the application",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		follow, err := cmd.Flags().GetBool("follow")
		if err != nil {
			return errors.Wrap(err, "error reading option --follow")
		}

		staging, err := cmd.Flags().GetBool("staging")
		if err != nil {
			return errors.Wrap(err, "error reading option --staging")
		}

		stageID, err := client.AppStageID(args[0])
		if err != nil {
			return errors.Wrap(err, "error checking app")
		}
		if staging {
			follow = false
		} else {
			stageID = ""
		}

		err = client.AppLogs(args[0], stageID, follow, nil)
		if err != nil {
			return errors.Wrap(err, "error streaming application logs")
		}

		return nil
	},
}

// CmdAppUpdate implements the command: epinio apps update
// It scales the named app
var CmdAppUpdate = &cobra.Command{
	Use:   "update NAME",
	Short: "Update the named application",
	Long:  "Update the running application's attributes (e.g. instances)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		m, err := manifest.UpdateISE(models.ApplicationManifest{}, cmd)
		if err != nil {
			return errors.Wrap(err, "unable to get app configuration")
		}

		m, err = manifest.UpdateDomains(m, cmd)
		if err != nil {
			return errors.Wrap(err, "unable to update domains")
		}

		err = client.AppUpdate(args[0], m.Configuration)
		if err != nil {
			return errors.Wrap(err, "error updating the app")
		}

		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := usercmd.New()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := app.AppsMatching(context.Background(), toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

// CmdAppManifest implements the command: epinio apps manifest
var CmdAppManifest = &cobra.Command{
	Use:   "manifest NAME MANIFESTPATH",
	Short: "Save state of the named application as a manifest",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.AppManifest(args[0], args[1])
		if err != nil {
			return errors.Wrap(err, "error getting app manifest")
		}

		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := usercmd.New()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := app.AppsMatching(context.Background(), toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}
