package kubernetes

import (
	"errors"
	"fmt"
)

const (
	BooleanType = iota
	StringType
	IntType
)

type InstallationOptionType int

type InstallationOption struct {
	Name         string
	Value        interface{}
	Description  string
	Type         InstallationOptionType
	DeploymentID string // If set, this option will be passed only to this deployment (private)
}

type InstallationOptions []InstallationOption

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

func (opts InstallationOptions) GetString(optionName string, deploymentID string) (string, error) {
	for _, option := range opts {
		if option.Name == optionName && option.DeploymentID == deploymentID {
			result, ok := option.Value.(string)
			if !ok {
				panic("wrong type assertion")
			} else {
				return result, nil
			}
		}
	}

	return "", errors.New(optionName + " not set")
}

func (opts InstallationOptions) GetBool(optionName string, deploymentID string) (bool, error) {
	for _, option := range opts {
		if option.Name == optionName && option.DeploymentID == deploymentID {
			result, ok := option.Value.(bool)
			if !ok {
				panic("wrong type assertion")
			} else {
				return result, nil
			}
		}
	}

	return false, errors.New(optionName + " not set")
}

func (opts InstallationOptions) GetInt(optionName string, deploymentID string) (int, error) {
	for _, option := range opts {
		if option.Name == optionName && option.DeploymentID == deploymentID {
			result, ok := option.Value.(int)
			if !ok {
				panic("wrong type assertion")
			} else {
				return result, nil
			}
		}
	}

	return 0, errors.New(optionName + " not set")
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
