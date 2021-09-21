package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	CmdServiceCreate.Flags().String("data", "", "json data to be passed to the underlying service as parameters")
	CmdServiceCreate.Flags().Bool("dont-wait", false, "Return immediately, without waiting for the service to be provisioned")
	CmdServiceDelete.Flags().Bool("unbind", false, "Unbind from applications before deleting")
	CmdService.AddCommand(CmdServiceShow)
	CmdService.AddCommand(CmdServiceCreate)
	CmdService.AddCommand(CmdServiceCreateCustom)
	CmdService.AddCommand(CmdServiceDelete)
	CmdService.AddCommand(CmdServiceBind)
	CmdService.AddCommand(CmdServiceUnbind)
	CmdService.AddCommand(CmdServiceListClasses)
	CmdService.AddCommand(CmdServiceListPlans)
	CmdService.AddCommand(CmdServiceList)
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
	Use:   "create NAME CLASS PLAN",
	Short: "Create a service",
	Long:  `Create service by name, class, plan, and optional json data.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 3 {
			return errors.New("Not enough arguments, expected name, class, plan")
		}
		return nil
	},
	RunE: ServiceCreate,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 2 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		if len(args) == 0 {
			// #args == 0: service name. new. nothing to match
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := usercmd.New()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		if len(args) == 2 {
			// #args == 2: service plan name.
			matches := app.ServicePlanMatching(context.Background(), args[1], toComplete)
			return matches, cobra.ShellCompDirectiveNoFileComp
		}

		// #args == 1: service class name.
		matches := app.ServiceClassMatching(context.Background(), toComplete)
		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

// CmdServiceCreateCustom implements the command: epinio service create-custom
var CmdServiceCreateCustom = &cobra.Command{
	Use:   "create-custom NAME (KEY VALUE)...",
	Short: "Create a custom service",
	Long:  `Create custom service by name and key/value dictionary.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 3 {
			return errors.New("Not enough arguments, expected name, key, and value")
		}
		if len(args)%2 == 0 {
			return errors.New("Last Key has no value")
		}
		return nil
	},
	RunE: ServiceCreateCustom,
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
			matches := app.AppsMatching(context.Background(), toComplete)
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
			matches := app.AppsMatching(context.Background(), toComplete)
			return matches, cobra.ShellCompDirectiveNoFileComp
		}

		// #args == 0: service name.
		matches := app.ServiceMatching(context.Background(), toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

// CmdServiceListClasses implements the command: epinio service classes
var CmdServiceListClasses = &cobra.Command{
	Use:   "list-classes",
	Short: "Lists the available service classes",
	RunE:  ServiceListClasses,
}

// CmdServiceListPlans implements the command: epinio service plans
var CmdServiceListPlans = &cobra.Command{
	Use:   "list-plans CLASS",
	Short: "Lists all plans provided by the named service class",
	Args:  cobra.ExactArgs(1),
	RunE:  ServiceListPlans,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := usercmd.New()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := app.ServiceClassMatching(context.Background(), toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}

// CmdServiceList implements the command: epinio service list
var CmdServiceList = &cobra.Command{
	Use:   "list",
	Short: "Lists all services",
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

	err = client.Services()
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

	dw, err := cmd.Flags().GetBool("dont-wait")
	if err != nil {
		return errors.Wrap(err, "error reading option --dont-wait")
	}
	waitforProvision := !dw

	data, err := cmd.Flags().GetString("data")
	if err != nil {
		return errors.Wrap(err, "error reading option --data")
	}

	if data == "" {
		data = "{}"
	}

	var dataObj map[string]interface{}
	err = json.Unmarshal([]byte(data), &dataObj)
	if err != nil {
		// User error. Show usage for this one.
		cmd.SilenceUsage = false
		return errors.Wrap(err, "Invalid json format for data")
	}

	err = client.CreateService(args[0], args[1], args[2], data, waitforProvision)
	if err != nil {
		return errors.Wrap(err, "error creating service")
	}

	return nil
}

// ServiceCreateCustom is the backend of command: epinio service create-custom
func ServiceCreateCustom(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	client, err := usercmd.New()
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = client.CreateCustomService(args[0], args[1:])
	if err != nil {
		return errors.Wrap(err, "error creating custom service")
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

// ServiceListClasses is the backend of command: epinio service list-classes
func ServiceListClasses(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	client, err := usercmd.New()
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = client.ServiceClasses()
	if err != nil {
		return errors.Wrap(err, "error listing service classes")
	}

	return nil
}

// ServiceListPlans is the backend of command: epinio service list-plans
func ServiceListPlans(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	client, err := usercmd.New()
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = client.ServicePlans(args[0])
	if err != nil {
		return errors.Wrap(err, "error listing plan")
	}

	return nil
}
