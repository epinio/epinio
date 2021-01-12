package kubernetes

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	BooleanType = iota
	StringType
	IntType
)

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

func (opts InstallationOptions) AsCobraFlagsFor(cmd *cobra.Command) {
	for _, opt := range opts {
		// Translate option name
		flagName := strings.ReplaceAll(opt.Name, "_", "-")

		// Declare option's flag, type-dependent
		switch opt.Type {
		case BooleanType:
			cmd.Flags().Bool(flagName, opt.Default.(bool), opt.Description)
		case StringType:
			cmd.Flags().String(flagName, opt.Default.(string), opt.Description)
		case IntType:
			cmd.Flags().Int(flagName, opt.Default.(int), opt.Description)
		}
	}
	return
}

func (opts InstallationOptions) ToOptMap() map[string]InstallationOption {
	result := map[string]InstallationOption{}
	for _, opt := range opts {
		result[opt.ToOptMapKey()] = opt
	}

	return result
}

func (opt InstallationOption) ToOptMapKey() string {
	return fmt.Sprintf("%s-%s", opt.Name, opt.DeploymentID)
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

// Merge returns a merge of the two options respecting uniqueness of name+deploymentID
func (opts InstallationOptions) Merge(toMerge InstallationOptions) InstallationOptions {
	result := InstallationOptions{}
	optMap := opts.ToOptMap()
	for _, mergeOpt := range toMerge {
		optMap[mergeOpt.ToOptMapKey()] = mergeOpt
	}

	for _, v := range optMap {
		result = append(result, v)
	}

	return result
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

func (opts InstallationOptions) ForDeployment(deploymentID string) InstallationOptions {
	result := InstallationOptions{}
	for _, opt := range opts {
		if opt.DeploymentID == deploymentID || opt.DeploymentID == "" {
			result = append(result, opt)
		}
	}

	return result
}
