package cli

import (
	"fmt"
	"strconv"

	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/cli/admincmd"
	"github.com/epinio/epinio/internal/cli/config"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

// CmdConfig implements the command: epinio config
var CmdConfig = &cobra.Command{
	Use:           "config",
	Short:         "Epinio config management",
	Long:          `Manage the epinio cli configuration`,
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
	CmdConfig.AddCommand(CmdConfigUpdate)
	CmdConfig.AddCommand(CmdConfigShow)
	CmdConfig.AddCommand(CmdConfigColors)
}

// CmdConfigColors implements the command: epinio config colors
var CmdConfigColors = &cobra.Command{
	Use:   "colors BOOL",
	Short: "Manage colored output",
	Long:  "Enable/Disable colored output",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("requires a boolean argument")
		}
		_, err := strconv.ParseBool(args[0])
		if err != nil {
			return errors.New("requires a boolean argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		ui := termui.NewUI()

		theConfig, err := config.Load()
		if err != nil {
			return errors.Wrap(err, "failed to load configuration")
		}

		ui.Note().WithStringValue("Config", theConfig.Location).Msg("Edit Colorization Flag")

		colors, err := strconv.ParseBool(args[0])
		// assert: err == nil -- see args validation
		if err != nil {
			return errors.Wrap(err, "unexpected bool parsing error")
		}

		theConfig.Colors = colors
		if err := theConfig.Save(); err != nil {
			return err
		}

		ui.Success().WithBoolValue("Colors", theConfig.Colors).Msg("Ok")
		return nil
	},
}

// CmdConfigShow implements the command: epinio config show
var CmdConfigShow = &cobra.Command{
	Use:   "show",
	Short: "Show the current configuration",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		ui := termui.NewUI()

		theConfig, err := config.Load()
		if err != nil {
			return errors.Wrap(err, "failed to load configuration")
		}

		ui.Note().WithStringValue("Config", theConfig.Location).Msg("Show Configuration")

		certInfo := color.CyanString("None defined")
		if theConfig.Certs != "" {
			certInfo = color.BlueString("Present")
		}

		ui.Success().
			WithTable("Key", "Value").
			WithTableRow("Colorized Output", color.MagentaString("%t", theConfig.Colors)).
			WithTableRow("Current Namespace", color.CyanString(theConfig.Org)).
			WithTableRow("API User Name", color.BlueString(theConfig.User)).
			WithTableRow("API Password", color.BlueString(theConfig.Password)).
			WithTableRow("API Url", color.BlueString(theConfig.API)).
			WithTableRow("WSS Url", color.BlueString(theConfig.WSS)).
			WithTableRow("Certificates", certInfo).
			Msg("Ok")

		return nil
	},
}

// CmdConfigUpdate implements the command: epinio config update
var CmdConfigUpdate = &cobra.Command{
	Use:   "update",
	Short: "Update the api location & stored credentials",
	Long:  "Update the api location and stored credentials from the current cluster",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := admincmd.New()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.ConfigUpdate(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "failed to update the configuration")
		}

		return nil
	},
}
