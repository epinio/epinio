package kubernetes

type DefaultOptionsReader struct{}

// NewDefaultOptionsReader is a reader used by the Installer to
// fill InstallationOptions with an initial default value, if such
// is specified (dynamic per function, or static).
func NewDefaultOptionsReader() DefaultOptionsReader {
	return DefaultOptionsReader{}
}

// Read attempts to fill the option with a default, dynamic or static.
func (reader DefaultOptionsReader) Read(option *InstallationOption) error {
	// Give priority to a function which provides the default
	// value dynamically.
	if option.DynDefault != nil {
		err := option.DynDefault(option)
		if err != nil {
			return err
		}
	} else if option.Default != nil {
		option.Value = option.Default
		option.Valid = true
	}

	return nil
}
