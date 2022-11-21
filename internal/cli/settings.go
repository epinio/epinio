package cli

import (
	"fmt"
	"strconv"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/cli/admincmd"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var ()

// CmdSettings implements the command: epinio settings
var CmdSettings = &cobra.Command{
	Use:           "settings",
	Short:         "Epinio settings management",
	Long:          `Manage the epinio cli settings`,
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
	flags := CmdSettingsUpdateCA.Flags()
	flags.StringP("namespace", "n", "epinio", "(NAMESPACE) The namespace to use")
	err := viper.BindPFlag("namespace", flags.Lookup("namespace"))
	checkErr(err)
	err = viper.BindEnv("namespace", "NAMESPACE")
	checkErr(err)

	CmdSettingsShow.Flags().Bool("show-password", false, "Show hidden password")
	err = viper.BindPFlag("show-password", CmdSettingsShow.Flags().Lookup("show-password"))
	checkErr(err)
	CmdSettingsShow.Flags().Bool("show-token", false, "Show access token")
	err = viper.BindPFlag("show-token", CmdSettingsShow.Flags().Lookup("show-token"))
	checkErr(err)

	CmdSettings.AddCommand(CmdSettingsUpdateCA)
	CmdSettings.AddCommand(CmdSettingsShow)
	CmdSettings.AddCommand(CmdSettingsColors)
}

// CmdSettingsColors implements the command: epinio settings colors
var CmdSettingsColors = &cobra.Command{
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

		theSettings, err := settings.Load()
		if err != nil {
			return errors.Wrap(err, "failed to load settings")
		}

		ui.Note().
			WithStringValue("Settings", helpers.AbsPath(theSettings.Location)).
			Msg("Edit Colorization Flag")

		colors, err := strconv.ParseBool(args[0])
		// assert: err == nil -- see args validation
		if err != nil {
			return errors.Wrap(err, "unexpected bool parsing error")
		}

		theSettings.Colors = colors
		if err := theSettings.Save(); err != nil {
			return err
		}

		ui.Success().WithBoolValue("Colors", theSettings.Colors).Msg("Ok")
		return nil
	},
}

// CmdSettingsShow implements the command: epinio settings show
var CmdSettingsShow = &cobra.Command{
	Use:   "show",
	Short: "Show the current settings",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		ui := termui.NewUI()

		theSettings, err := settings.Load()
		if err != nil {
			return errors.Wrap(err, "failed to load settings")
		}

		ui.Note().
			WithStringValue("Settings", helpers.AbsPath(theSettings.Location)).
			Msg("Show Settings")

		certInfo := color.CyanString("None defined")
		if theSettings.Certs != "" {
			certInfo = color.BlueString("Present")
		}

		var password string
		if theSettings.Password != "" {
			password = "***********"
			if viper.GetBool("show-password") {
				password = theSettings.Password
			}
		}

		var token string
		if theSettings.Token.AccessToken != "" {
			token = "***********"
			if viper.GetBool("show-token") {
				token = theSettings.Token.AccessToken
			}
		}

		ui.Success().
			WithTable("Key", "Value").
			WithTableRow("Colorized Output", color.MagentaString("%t", theSettings.Colors)).
			WithTableRow("Current Namespace", color.CyanString(theSettings.Namespace)).
			WithTableRow("Default App Chart", color.CyanString(theSettings.AppChart)).
			WithTableRow("API User Name", color.BlueString(theSettings.User)).
			WithTableRow("API Password", color.BlueString(password)).
			WithTableRow("API Token", color.BlueString(token)).
			WithTableRow("API Url", color.BlueString(theSettings.API)).
			WithTableRow("WSS Url", color.BlueString(theSettings.WSS)).
			WithTableRow("Certificates", certInfo).
			Msg("Ok")

		return nil
	},
}

// CmdSettingsUpdateCA implements the command: epinio settings update-ca
var CmdSettingsUpdateCA = &cobra.Command{
	Use:   "update-ca",
	Short: "Update the api location and CA certificate",
	Long:  "Update the api location and CA certificate from the current cluster",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := admincmd.New()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.SettingsUpdateCA(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "failed to update the settings")
		}

		return nil
	},
}
