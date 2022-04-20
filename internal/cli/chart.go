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
	CmdAppChart.AddCommand(CmdAppChartDefault)

	// Create: --short, --desc, --helm-repo
	CmdAppChartCreate.Flags().StringP("short", "s", "", "Short description of the new chart")
	CmdAppChartCreate.Flags().StringP("desc", "d", "", "Long description of the new chart")
	CmdAppChartCreate.Flags().StringP("helm-repo", "r", "", "Helm repository needed to resolve the chart ref")

}

// CmdAppChartDefault implements the command: epinio app chart default
var CmdAppChartDefault = &cobra.Command{
	Use:               "default [CHARTNAME]",
	Short:             "Set or show app chart default",
	Long:              "Set or show app chart default",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: matchingChartFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		if len(args) == 1 {
			err = client.ChartDefaultSet(cmd.Context(), args[0])
			if err != nil {
				return errors.Wrap(err, "error setting app chart default")
			}
		} else {
			err = client.ChartDefaultShow(cmd.Context())
			if err != nil {
				return errors.Wrap(err, "error showing app chart default")
			}
		}

		return nil
	},
}

// CmdAppChartList implements the command: epinio app chart list
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
	Use:   "create [OPTIONS] NAME CHARTREF",
	Short: "Extend set of application charts",
	Long:  "Make new application chart known to epinio",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		short, err := cmd.Flags().GetString("short")
		if err != nil {
			return errors.Wrap(err, "error reading option --short")
		}
		desc, err := cmd.Flags().GetString("desc")
		if err != nil {
			return errors.Wrap(err, "error reading option --desc")
		}
		repo, err := cmd.Flags().GetString("helm-repo")
		if err != nil {
			return errors.Wrap(err, "error reading option --helm-repo")
		}


		err = client.ChartCreate(cmd.Context(), args[0], args[1], short, desc, repo)
		if err != nil {
			return errors.Wrap(err, "error creating chart")
		}

		return nil
	},
}

// CmdAppChartShow implements the command: epinio app env show
var CmdAppChartShow = &cobra.Command{
	Use:               "show CHARTNAME",
	Short:             "Describe application chart",
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
			return errors.Wrap(err, "error showing app chart")
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
			return errors.Wrap(err, "error removing app chart")
		}

		return nil
	},
}
