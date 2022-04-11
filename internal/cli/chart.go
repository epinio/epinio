package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// CmdAppChart implements the command: epinio app chart
var CmdAppChart = &cobra.Command{
	Use:   "chart",
	Short: "Epinio application chart management",
	Long:  `Manage epinio application charts`,
}

func init() {
	CmdAppChart.AddCommand(CmdAppChartList)
	CmdAppChart.AddCommand(CmdAppChartCreate)
	CmdAppChart.AddCommand(CmdAppChartShow)
	CmdAppChart.AddCommand(CmdAppChartDelete)
}

// CmdAppChartList implements the command: epinio app env list
var CmdAppChartList = &cobra.Command{
	Use:   "list",
	Short: "List application charts",
	Long:  "List applications charts",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.ChartList(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error listing app charts")
		}

		return nil
	},
}

// CmdAppChartCreate implements the command: epinio app chart create
var CmdAppChartCreate = &cobra.Command{
	Use:   "create NAME --repo REPO URL",
	Short: "Extend set of application charts",
	Long:  "Make a new application chart known to epinio",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		// TODO: process options, pass options

		err = client.ChartCreate(cmd.Context(), args[0], args[1], "<<TODO:repo>>")
		if err != nil {
			return errors.Wrap(err, "error setting into app environment")
		}

		return nil
	},
}

// CmdAppChartShow implements the command: epinio app env show
var CmdAppChartShow = &cobra.Command{
	Use:               "show CHARTNAME",
	Short:             "Describe application's environment variable",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: matchingChartFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.ChartShow(cmd.Context(), args[0])
		if err != nil {
			return errors.Wrap(err, "error accessing app environment")
		}

		return nil
	},
}

// CmdAppChartDelete implements the command: epinio app env unset
var CmdAppChartDelete = &cobra.Command{
	Use:               "delete CHARTNAME",
	Short:             "Remove application chart from epinio",
	Long:              "Remove application chart from epinio",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: matchingChartFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.ChartDelete(cmd.Context(), args[0])
		if err != nil {
			return errors.Wrap(err, "error removing from app environment")
		}

		return nil
	},
}
