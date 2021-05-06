package cli

import (
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

// CmdOrg implements the epinio -app command
var CmdOrg = &cobra.Command{
	Use:           "org",
	Aliases:       []string{"orgs"},
	Short:         "Epinio organizations",
	Long:          `Manage epinio organizations`,
	Args:          cobra.ExactArgs(0),
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	CmdOrg.AddCommand(CmdOrgCreate)
	CmdOrg.AddCommand(CmdOrgList)
	CmdOrg.AddCommand(CmdOrgDelete)
}

// CmdOrgs implements the epinio `orgs list` command
var CmdOrgList = &cobra.Command{
	Use:   "list",
	Short: "Lists all organizations",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clients.NewEpinioClient(cmd.Flags())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.Orgs()
		if err != nil {
			return errors.Wrap(err, "error listing orgs")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

// CmdOrgCreate implements the epinio `orgs create` command
var CmdOrgCreate = &cobra.Command{
	Use:   "create NAME",
	Short: "Creates an organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clients.NewEpinioClient(cmd.Flags())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.CreateOrg(args[0])
		if err != nil {
			return errors.Wrap(err, "error creating org")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

// CmdOrgDelete implements the epinio `orgs delete` command
var CmdOrgDelete = &cobra.Command{
	Use:   "delete NAME",
	Short: "Deletes an organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clients.NewEpinioClient(cmd.Flags())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.DeleteOrg(args[0])
		if err != nil {
			return errors.Wrap(err, "error deleting org")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}
