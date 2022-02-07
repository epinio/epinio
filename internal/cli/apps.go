package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/internal/manifest"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// CmdApp implements the  command: epinio app
var CmdApp = &cobra.Command{
	Use:           "app",
	Aliases:       []string{"apps"},
	Short:         "Epinio application features",
	Long:          `Manage epinio application`,
	SilenceErrors: false,
	Args:          cobra.MinimumNArgs(1),
}

func init() {
	CmdAppList.Flags().Bool("all", false, "list all applications")
	CmdAppLogs.Flags().Bool("follow", false, "follow the logs of the application")
	CmdAppLogs.Flags().Bool("staging", false, "show the staging logs of the application")
	CmdAppExec.Flags().StringP("instance", "i", "", "The name of the instance to shell to")
	CmdAppPortForward.Flags().StringSliceVar(&portForwardAddress, "address", []string{"localhost"}, "Addresses to listen on (comma separated). Only accepts IP addresses or localhost as a value. When localhost is supplied, kubectl will try to bind on both 127.0.0.1 and ::1 and will fail if neither of these addresses are available to bind.")
	CmdAppPortForward.Flags().StringVarP(&portForwardInstance, "instance", "i", "", "The name of the instance to shell to")

	routeOption(CmdAppCreate)
	routeOption(CmdAppUpdate)
	bindOption(CmdAppCreate)
	bindOption(CmdAppUpdate)
	envOption(CmdAppCreate)
	envOption(CmdAppUpdate)
	instancesOption(CmdAppCreate)
	instancesOption(CmdAppUpdate)

	CmdApp.AddCommand(CmdAppCreate)
	CmdApp.AddCommand(CmdAppEnv) // See env.go for implementation
	CmdApp.AddCommand(CmdAppList)
	CmdApp.AddCommand(CmdAppLogs)
	CmdApp.AddCommand(CmdAppExec)
	CmdApp.AddCommand(CmdAppPortForward)

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
		// Note: errors.Wrap (nil, "...") == nil
		return errors.Wrap(err, "error listing apps")
	},
}

// CmdAppCreate implements the command: epinio apps create
var CmdAppCreate = &cobra.Command{
	Use:               "create NAME",
	Short:             "Create just the app, without creating a workload",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: matchingAppsFinder,
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

		m, err = manifest.UpdateRoutes(m, cmd)
		if err != nil {
			return err
		}

		err = client.AppCreate(args[0], m.Configuration)
		// Note: errors.Wrap (nil, "...") == nil
		return errors.Wrap(err, "error creating app")
	},
}

// CmdAppShow implements the command: epinio apps show
var CmdAppShow = &cobra.Command{
	Use:               "show NAME",
	Short:             "Describe the named application",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: matchingAppsFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.AppShow(args[0])
		// Note: errors.Wrap (nil, "...") == nil
		return errors.Wrap(err, "error showing app")
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
		// Note: errors.Wrap (nil, "...") == nil
		return errors.Wrap(err, "error streaming application logs")
	},
}

// CmdAppExec implements the command: epinio apps exec
var CmdAppExec = &cobra.Command{
	Use:   "exec NAME",
	Short: "creates a shell to the application",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		instance, err := cmd.Flags().GetString("instance")
		if err != nil {
			cmd.SilenceUsage = false
			return errors.Wrap(err, "could not read instance parameter")
		}

		err = client.AppExec(cmd.Context(), args[0], instance)
		// Note: errors.Wrap (nil, "...") == nil
		return errors.Wrap(err, "error getting a shell to application")
	},
}

var (
	portForwardAddress  []string
	portForwardInstance string
)

// CmdAppPortForward implements the command: epinio apps port-forward
var CmdAppPortForward = &cobra.Command{
	Use:   "port-forward NAME [LOCAL_PORT:]REMOTE_PORT [...[LOCAL_PORT_N:]REMOTE_PORT_N]",
	Short: "creates a shell to the application",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		appName := args[0]
		ports := args[1:]

		err = client.AppPortForward(cmd.Context(), appName, portForwardInstance, portForwardAddress, ports)
		// Note: errors.Wrap (nil, "...") == nil
		return errors.Wrap(err, "error port forwarding to application")
	},
}

// CmdAppUpdate implements the command: epinio apps update
// It scales the named app
var CmdAppUpdate = &cobra.Command{
	Use:               "update NAME",
	Short:             "Update the named application",
	Long:              "Update the running application's attributes (e.g. instances)",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: matchingAppsFinder,
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

		m, err = manifest.UpdateRoutes(m, cmd)
		if err != nil {
			return errors.Wrap(err, "unable to update domains")
		}

		err = client.AppUpdate(args[0], m.Configuration)
		// Note: errors.Wrap (nil, "...") == nil
		return errors.Wrap(err, "error updating the app")
	},
}

// CmdAppManifest implements the command: epinio apps manifest
var CmdAppManifest = &cobra.Command{
	Use:               "manifest NAME MANIFESTPATH",
	Short:             "Save state of the named application as a manifest",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: matchingAppsFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.AppManifest(args[0], args[1])
		// Note: errors.Wrap (nil, "...") == nil
		return errors.Wrap(err, "error getting app manifest")
	},
}
