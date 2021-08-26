package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	force bool
)

// CmdOrg implements the command: epinio org
var CmdOrg = &cobra.Command{
	Use:           "org",
	Aliases:       []string{"orgs"},
	Short:         "Epinio organizations",
	Long:          `Manage epinio organizations`,
	SilenceErrors: true,
	SilenceUsage:  true,
	Args:          cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Usage()
		return fmt.Errorf(`Unknown method "%s"`, args[0])
	},
}

func init() {

	flags := CmdOrgDelete.Flags()
	flags.BoolVarP(&force, "force", "f", false, "force org deletion")

	CmdOrg.AddCommand(CmdOrgCreate)
	CmdOrg.AddCommand(CmdOrgList)
	CmdOrg.AddCommand(CmdOrgDelete)
}

// CmdOrgs implements the command: epinio org list
var CmdOrgList = &cobra.Command{
	Use:   "list",
	Short: "Lists all organizations",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := clients.NewEpinioClient(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.Orgs()
		if err != nil {
			return errors.Wrap(err, "error listing orgs")
		}

		return nil
	},
}

// CmdOrgCreate implements the command: epinio org create
var CmdOrgCreate = &cobra.Command{
	Use:   "create NAME",
	Short: "Creates an organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := clients.NewEpinioClient(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.CreateOrg(args[0])
		if err != nil {
			return errors.Wrap(err, "error creating org")
		}

		return nil
	},
}

// CmdOrgDelete implements the command: epinio org delete
var CmdOrgDelete = &cobra.Command{
	Use:   "delete NAME",
	Short: "Deletes an organization",
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return err
		}
		if !force {
			cmd.Printf("You are about to delete organization %s and everything included in it (applications, services etc). Are you sure? (y/n): ", args[0])
			if !askConfirmation(cmd) {
				return errors.New("Cancelled by user")
			}
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := clients.NewEpinioClient(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.DeleteOrg(args[0])
		if err != nil {
			return errors.Wrap(err, "error deleting org")
		}

		return nil
	},
}

// askConfirmation is a helper for CmdOrgDelete to confirm a deletion request
func askConfirmation(cmd *cobra.Command) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		s, _ := reader.ReadString('\n')
		s = strings.TrimSuffix(s, "\n")
		s = strings.ToLower(s)
		if strings.Compare(s, "n") == 0 {
			return false
		} else if strings.Compare(s, "y") == 0 {
			break
		} else {
			cmd.Printf("Please enter y or n: ")
			continue
		}
	}
	return true
}
