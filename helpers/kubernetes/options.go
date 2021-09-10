package kubernetes

import (
	"errors"
	"strings"

	"github.com/spf13/pflag"
)

const (
	BooleanType = iota
	StringType
	IntType
)

// OptionsReader is the interface to the structures and objects used
// to fill InstallationOption instances with a valid value.
//
// Note, each reader has the discretion to not modify the provided
// option instance based on its state. The option's Valid flag is, for
// example, how the defaults, cli, and interactive readers communicate
// and decide which options to handle.
type OptionsReader interface {
	Read(*InstallationOption) error
}

// A InstallationOptionDynamicDefault function may provide a dynamic
// default value for an option. When present it has precedence over
// any static default value in the structure.
//
// ATTENTION: The function is responsible for setting both Value and
// Valid flag of the specified option. This is necessary for cases
// where the dynamic default could not be determined, yet is not an
// error.
//
type InstallationOptionDynamicDefault func(o *InstallationOption) error

type InstallationOptionType int

type InstallationOption struct {
	Name           string                           // Identifying name of the configuration variable
	Value          interface{}                      // Value to use (may not be valid, see `Valid` field).
	Default        interface{}                      // Static default value for the value.
	DynDefaultFunc InstallationOptionDynamicDefault // Function to provide a default. Has priority over `Default`.
	UserSpecified  bool                             // Flag, true if `Value` came from the user.
	Description    string                           // Short description of the variable
	Type           InstallationOptionType           // Type information for `Value` and `Default`.
	DeploymentID   string                           // If set, this option will be passed only to this deployment (private)
}

type InstallationOptions []InstallationOption

func (opts InstallationOptions) AsCobraFlagsFor(flags *pflag.FlagSet) {
	for _, opt := range opts {
		// Translate option name
		flagName := strings.ReplaceAll(opt.Name, "_", "-")

		// Declare option's flag, type-dependent
		switch opt.Type {
		case BooleanType:
			if opt.Default == nil {
				flags.Bool(flagName, false, opt.Description)
			} else {
				flags.Bool(flagName, opt.Default.(bool), opt.Description)
			}
		case StringType:
			if opt.Default == nil {
				flags.String(flagName, "", opt.Description)
			} else {
				flags.String(flagName, opt.Default.(string), opt.Description)
			}
		case IntType:
			if opt.Default == nil {
				flags.Int(flagName, 0, opt.Description)
			} else {
				flags.Int(flagName, opt.Default.(int), opt.Description)
			}
		}
	}
}

func (opt *InstallationOption) DynDefault() error {
	return opt.DynDefaultFunc(opt)
}

func (opt *InstallationOption) SetDefault() error {
	// Give priority to a function which provides the default
	// value dynamically.
	if opt.DynDefaultFunc != nil {
		err := opt.DynDefault()
		if err != nil {
			return err
		}
	} else if opt.Default != nil {
		opt.Value = opt.Default
	}

	return nil
}

// GetOpt finds the given option in opts.
//
// When the deploymentID is the empty string the function searches for
// and returns only global options (not associated to any deployment).
// Otherwise it searches for private options associated with the
// specified deployment as well.
//
// ATTENTION: In the second case private options have precedence. In
// other words if we have private and global options of the same name,
// then the private option is returned.
//
// ATTENTION: This function returns a reference, enabling the caller
// to modify the structure.
func (opts InstallationOptions) GetOpt(optionName string, deploymentID string) (*InstallationOption, error) {
	if deploymentID != "" {
		// "Private" options first, only if a deployment to search is known
		for i, option := range opts {
			if option.Name == optionName && option.DeploymentID == deploymentID {
				return &opts[i], nil
			}
		}
	}

	// If there is no private option, try "Global" options
	for i, option := range opts {
		if option.Name == optionName && option.DeploymentID == "" {
			return &opts[i], nil
		}
	}

	return nil, errors.New(optionName + " not set")
}

func (opts InstallationOptions) GetString(optionName string, deploymentID string) (string, error) {
	option, err := opts.GetOpt(optionName, deploymentID)
	if err != nil {
		return "", err
	}

	result, ok := option.Value.(string)
	if !ok {
		panic("wrong type assertion")
	}

	return result, nil
}

func (opts InstallationOptions) GetBool(optionName string, deploymentID string) (bool, error) {
	option, err := opts.GetOpt(optionName, deploymentID)
	if err != nil {
		return false, err
	}

	result, ok := option.Value.(bool)
	if !ok {
		panic("wrong type assertion")
	}

	return result, nil
}

func (opts InstallationOptions) GetInt(optionName string, deploymentID string) (int, error) {
	option, err := opts.GetOpt(optionName, deploymentID)
	if err != nil {
		return 0, err
	}

	result, ok := option.Value.(int)
	if !ok {
		panic("wrong type assertion")
	}

	return result, nil
}

// GetStringNG returns the string value for a needed, global option
func (opts InstallationOptions) GetStringNG(optionName string) string {
	option, err := opts.GetOpt(optionName, "")
	if err != nil {
		return ""
	}
	result, ok := option.Value.(string)
	if !ok {
		return ""
	}
	return result
}

// GetBoolNG returns the bool value for a needed, global option
func (opts InstallationOptions) GetBoolNG(optionName string) bool {
	option, err := opts.GetOpt(optionName, "")
	if err != nil {
		return false
	}
	result, ok := option.Value.(bool)
	if !ok {
		return false
	}
	return result
}

func (opts InstallationOptions) ForDeployment(deploymentID string) InstallationOptions {
	result := InstallationOptions{}
	for _, opt := range opts {
		if opt.DeploymentID == deploymentID || opt.DeploymentID == "" {
			result = append(result, opt)
		}
	}

	return result
}

// Populate will try to give values to the needed options
// using the given OptionsReader. If none is given, the default is the
// InteractiveOptionsReader which will ask in the terminal.
// This method only populates what is possible and leaves the rest empty.
// TODO: Implement another method to validate that all options have been set.
func (opts *InstallationOptions) Populate(reader OptionsReader) (*InstallationOptions, error) {
	newOpts := InstallationOptions{}
	for _, opt := range *opts {
		newopt := opt
		err := reader.Read(&newopt)
		if err != nil {
			return opts, err
		}
		newOpts = append(newOpts, newopt)
	}

	return &newOpts, nil
}
