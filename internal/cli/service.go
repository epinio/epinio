package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	CmdServiceDelete.Flags().Bool("unbind", false, "Unbind from applications before deleting")
	CmdService.AddCommand(CmdServiceShow)
	CmdService.AddCommand(CmdServiceCreate)
	CmdService.AddCommand(CmdServiceUpdate)
	CmdService.AddCommand(CmdServiceDelete)
	CmdService.AddCommand(CmdServiceBind)
	CmdService.AddCommand(CmdServiceUnbind)
	CmdService.AddCommand(CmdServiceList)

	CmdServiceList.Flags().Bool("all", false, "list all services")

	changeOptions(CmdServiceUpdate)
}

// CmdService implements the command: epinio service
var CmdService = &cobra.Command{
	Use:           "service",
	Aliases:       []string{"services"},
	Short:         "Epinio service features",
	Long:          `Handle service features with Epinio`,
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

// CmdServiceShow implements the command: epinio service show
var CmdServiceShow = &cobra.Command{
	Use:   "show NAME",
	Short: "Service information",
	Long:  `Show detailed information of the named service.`,
	Args:  cobra.ExactArgs(1),
	RunE:  ServiceShow,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := usercmd.New()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := app.ServiceMatching(context.Background(), toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

// CmdServiceCreate implements the command: epinio service create
var CmdServiceCreate = &cobra.Command{
	Use:   "create NAME (KEY VALUE)...",
	Short: "Create a service",
	Long:  `Create service by name and key/value dictionary.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 3 {
			return errors.New("Not enough arguments, expected name, key, and value")
		}
		if len(args)%2 == 0 {
			return errors.New("Last Key has no value")
		}
		return nil
	},
	RunE: ServiceCreate,
}

// CmdServiceUpdate implements the command: epinio service create
var CmdServiceUpdate = &cobra.Command{
	Use:   "update NAME",
	Short: "Update a service",
	Long:  `Update service by name and change instructions through flags.`,
	Args:  cobra.ExactArgs(1),
	RunE:  ServiceUpdate,
}

// CmdServiceDelete implements the command: epinio service delete
var CmdServiceDelete = &cobra.Command{
	Use:   "delete NAME",
	Short: "Delete a service",
	Long:  `Delete service by name.`,
	Args:  cobra.ExactArgs(1),
	RunE:  ServiceDelete,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		epinioClient, err := usercmd.New()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := epinioClient.ServiceMatching(context.Background(), toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

// CmdServiceBind implements the command: epinio service bind
var CmdServiceBind = &cobra.Command{
	Use:   "bind NAME APP",
	Short: "Bind a service to an application",
	Long:  `Bind service by name, to named application.`,
	Args:  cobra.ExactArgs(2),
	RunE:  ServiceBind,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 1 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := usercmd.New()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		if len(args) == 1 {
			// #args == 1: app name.
			matches := app.AppsMatching(toComplete)
			return matches, cobra.ShellCompDirectiveNoFileComp
		}

		// #args == 0: service name.

		matches := app.ServiceMatching(context.Background(), toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

// CmdServiceUnbind implements the command: epinio service unbind
var CmdServiceUnbind = &cobra.Command{
	Use:   "unbind NAME APP",
	Short: "Unbind service from an application",
	Long:  `Unbind service by name, from named application.`,
	Args:  cobra.ExactArgs(2),
	RunE:  ServiceUnbind,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 1 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := usercmd.New()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		if len(args) == 1 {
			// #args == 1: app name.
			matches := app.AppsMatching(toComplete)
			return matches, cobra.ShellCompDirectiveNoFileComp
		}

		// #args == 0: service name.
		matches := app.ServiceMatching(context.Background(), toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

// CmdServiceList implements the command: epinio service list
var CmdServiceList = &cobra.Command{
	Use:   "list [--all]",
	Short: "Lists services",
	Long:  "Lists services in the targeted namespace, or all",
	RunE:  ServiceList,
}

// ServiceShow is the backend of command: epinio service show
func ServiceShow(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	client, err := usercmd.New()
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = client.ServiceDetails(args[0])
	if err != nil {
		return errors.Wrap(err, "error retrieving service")
	}

	return nil
}

// ServiceList is the backend of command: epinio service list
func ServiceList(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	client, err := usercmd.New()
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return errors.Wrap(err, "error reading option --all")
	}

	err = client.Services(all)
	if err != nil {
		return errors.Wrap(err, "error listing services")
	}

	return nil
}

// ServiceCreate is the backend of command: epinio service create
func ServiceCreate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	client, err := usercmd.New()
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = client.CreateService(args[0], args[1:])
	if err != nil {
		return errors.Wrap(err, "error creating service")
	}

	return nil
}

// ServiceUpdate is the backend of command: epinio service update
func ServiceUpdate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	client, err := usercmd.New()
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	// Process the --remove and --set options into operations (removals, assignments)

	removedKeys, err := cmd.Flags().GetStringSlice("remove")
	if err != nil {
		return errors.Wrap(err, "failed to read option --remove")
	}

	kvAssignments, err := cmd.Flags().GetStringSlice("set")
	if err != nil {
		return errors.Wrap(err, "failed to read option --set")
	}

	assignments := map[string]string{}
	for _, assignment := range kvAssignments {
		pieces := strings.Split(assignment, "=")
		if len(pieces) != 2 {
			return errors.New("Bad --set assignment `" + assignment + "`, expected `name=value` as value")
		}
		assignments[pieces[0]] = pieces[1]
	}

	err = client.UpdateService(args[0], removedKeys, assignments)
	if err != nil {
		return errors.Wrap(err, "error creating service")
	}

	return nil
}

// ServiceDelete is the backend of command: epinio service delete
func ServiceDelete(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	unbind, err := cmd.Flags().GetBool("unbind")
	if err != nil {
		return errors.Wrap(err, "error reading option --unbind")
	}

	client, err := usercmd.New()
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = client.DeleteService(args[0], unbind)
	if err != nil {
		return errors.Wrap(err, "error deleting service")
	}

	return nil
}

// ServiceBind is the backend of command: epinio service bind
func ServiceBind(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	client, err := usercmd.New()
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = client.BindService(args[0], args[1])
	if err != nil {
		return errors.Wrap(err, "error binding service")
	}

	return nil
}

// ServiceUnbind is the backend of command: epinio service unbind
func ServiceUnbind(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	client, err := usercmd.New()
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = client.UnbindService(args[0], args[1])
	if err != nil {
		return errors.Wrap(err, "error unbinding service")
	}

	return nil
}

// changeOptions initializes the --remove/-r and --set/-s options for
// the provided command.
func changeOptions(cmd *cobra.Command) {
	cmd.Flags().StringSliceP("set", "s", []string{}, "service key/value assignments to add/modify")
	cmd.Flags().StringSliceP("remove", "r", []string{}, "service keys to remove")

	// Note: No completion functionality. This would require asking the service for
	// its details so that the keys to remove can be matched. And add/modify cannot
	// check anyway.
}
