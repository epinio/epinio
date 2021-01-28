package kubernetes

// Note. Defaults are set after all the other readers had their
// turn. It therefore has to skip every option which is already
// specified by the user.

type DefaultOptionsReader struct{}

// NewDefaultOptionsReader is a reader used by the Installer to fill
// InstallationOptions with a default value, either static, or dynamic
// per function vector.
func NewDefaultOptionsReader() DefaultOptionsReader {
	return DefaultOptionsReader{}
}

// Read attempts to fill the option with a default, dynamic or static.
func (reader DefaultOptionsReader) Read(option *InstallationOption) error {
	if option.UserSpecified {
		return nil
	}

	return option.SetDefault()
}
