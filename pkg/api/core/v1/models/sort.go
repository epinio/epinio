package models

// Implement the Sort interface for configuration response slices

// Len (Sort interface) returns the length of the ConfigurationResponseList
func (srl ConfigurationResponseList) Len() int {
	return len(srl)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the ConfigurationResponseList
func (srl ConfigurationResponseList) Swap(i, j int) {
	srl[i], srl[j] = srl[j], srl[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the ConfigurationResponseList and returns true if the
// condition holds, and else false.
func (srl ConfigurationResponseList) Less(i, j int) bool {
	// Comparison is done on the namespace names first, and then on the configuration names.
	return (srl[i].Meta.Namespace < srl[j].Meta.Namespace) ||
		((srl[i].Meta.Namespace == srl[j].Meta.Namespace) &&
			(srl[i].Meta.Name < srl[j].Meta.Name))
}
