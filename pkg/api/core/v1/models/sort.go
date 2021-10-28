package models

// Implement the Sort interface for service response slices

// Len (Sort interface) returns the length of the ServiceResponseList
func (srl ServiceResponseList) Len() int {
	return len(srl)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the ServiceResponseList
func (srl ServiceResponseList) Swap(i, j int) {
	srl[i], srl[j] = srl[j], srl[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the ServiceResponseList and returns true if the
// condition holds, and else false.
func (srl ServiceResponseList) Less(i, j int) bool {
	return srl[i].Meta.Name < srl[j].Meta.Name
}
