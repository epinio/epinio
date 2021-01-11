package kubernetes

import (
	"errors"
	"fmt"

	"github.com/kyokomi/emoji"
)

type CheckOptionsReader struct{}

// NewCheckOptionsReader is a reader used by the Installer to verify
// that all configuration variables have a valid value.
func NewCheckOptionsReader() CheckOptionsReader {
	return CheckOptionsReader{}
}

// Read checks that the specified InstallationOption is valid.
func (reader CheckOptionsReader) Read(option *InstallationOption) error {
	if !option.Valid {
		return errors.New(fmt.Sprintf("Option %s has no valid value", option.Name))
	}

	fmt.Printf("  %s%s:\t'%v'\n", emoji.Sprintf(":compass:"), option.Name, option.Value)
	return nil
}
