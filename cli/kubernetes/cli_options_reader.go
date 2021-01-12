package kubernetes

import (
	"strings"

	"github.com/spf13/cobra"
)

type CLIOptionsReader struct {
	cmd *cobra.Command
}

// NewCLIOptionsReader is a reader used by the Installer to fill
// configuration variables from cli options.
func NewCLIOptionsReader(cmd *cobra.Command) CLIOptionsReader {
	return CLIOptionsReader{cmd: cmd}
}

// Queries the cobra command for a flag associated with the given
// InstallationOption and returns its value converted to the
// appropriate (Go) type as defined by the Type field of the
// InstallationOption. Does nothing if no cobra flag is found.
func (reader CLIOptionsReader) Read(option *InstallationOption) error {
	// Translate option name
	flagName := strings.ReplaceAll(option.Name, "_", "-")

	// Get option value. The default is considered as `not set`,
	// forcing use of the interactive reader.  This is a hack I m
	// unhappy with as it means that the user cannot specify the
	// default, even if that is what they want.
	//
	// Unfortunately my quick look through the spf13/pflags
	// documentation has not shown me a way to properly determine
	// if a flag was not used on the command line at all,
	// vs. specified with (possibly the default) value.
	//
	// Do nothing if the specified option has no associated cobra
	// flag.

	if reader.cmd.Flags().Lookup(flagName) == nil {
		return nil
	}

	var cliValue interface{}
	var cliValid bool
	var err error

	switch option.Type {
	case BooleanType:
		cliValue, err = reader.cmd.Flags().GetBool(flagName)
		cliValid = (err != nil) || (cliValue.(bool) != option.Default.(bool))
	case StringType:
		cliValue, err = reader.cmd.Flags().GetString(flagName)
		cliValid = (err != nil) || (cliValue.(string) != option.Default.(string))
	case IntType:
		cliValue, err = reader.cmd.Flags().GetInt(flagName)
		cliValid = (err != nil) || (cliValue.(int) != option.Default.(int))
	}

	if err != nil {
		return err
	}

	if cliValid {
		option.Value = cliValue
		option.UserSpecified = true
	}

	return nil
}
