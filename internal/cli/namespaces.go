package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	force bool
)

// CmdNamespace implements the command: epinio namespace
var CmdNamespace = &cobra.Command{
	Use:           "namespace",
	Aliases:       []string{"namespaces"},
	Short:         "Epinio-controlled namespaces",
	Long:          `Manage epinio-controlled namespaces`,
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

	flags := CmdNamespaceDelete.Flags()
	flags.BoolVarP(&force, "force", "f", false, "force namespace deletion")

	CmdNamespace.AddCommand(CmdNamespaceCreate)
	CmdNamespace.AddCommand(CmdNamespaceList)
	CmdNamespace.AddCommand(CmdNamespaceDelete)
	CmdNamespace.AddCommand(CmdNamespaceShow)
}

// CmdNamespaces implements the command: epinio namespace list
var CmdNamespaceList = &cobra.Command{
	Use:   "list",
	Short: "Lists all epinio-controlled namespaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.Namespaces()
		if err != nil {
			return errors.Wrap(err, "error listing epinio-controlled namespaces")
		}

		return nil
	},
}

// CmdNamespaceCreate implements the command: epinio namespace create
var CmdNamespaceCreate = &cobra.Command{
	Use:   "create NAME",
	Short: "Creates an epinio-controlled namespace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.CreateNamespace(args[0])
		if err != nil {
			return errors.Wrap(err, "error creating epinio-controlled namespace")
		}

		return nil
	},
}

// CmdNamespaceDelete implements the command: epinio namespace delete
var CmdNamespaceDelete = &cobra.Command{
	Use:               "delete NAME",
	Short:             "Deletes an epinio-controlled namespace",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: matchingNamespaceFinder,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return err
		}
		if !force {
			cmd.Printf("You are about to delete namespace %s and everything it includes, i.e. applications, configurations, etc. Are you sure? (y/n): ", args[0])
			if !askConfirmation(cmd) {
				return errors.New("Cancelled by user")
			}
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.DeleteNamespace(args[0])
		if err != nil {
			return errors.Wrap(err, "error deleting epinio-controlled namespace")
		}

		return nil
	},
}

// CmdNamespaceShow implements the command: epinio namespace show
var CmdNamespaceShow = &cobra.Command{
	Use:               "show NAME",
	Short:             "Shows the details of an epinio-controlled namespace",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: matchingNamespaceFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.ShowNamespace(args[0])
		if err != nil {
			return errors.Wrap(err, "error showing epinio-controlled namespace")
		}

		return nil
	},
}

// askConfirmation is a helper for CmdNamespaceDelete to confirm a deletion request
func askConfirmation(cmd *cobra.Command) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		s, _ := reader.ReadString('\n')
		s = strings.TrimSpace(strings.ToLower(s))
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
