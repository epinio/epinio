package models

// Namespace has all the namespace properties, i.e. name, app names, and service names
// It is used in the CLI and API responses.
type Namespace struct {
	Name     string   `json:"name,omitempty"`
	Apps     []string `json:"apps,omitempty"`
	Services []string `json:"services,omitempty"`
}

// NamespaceList is a collection of namespaces
type NamespaceList []Namespace

// Implement the Sort interface for namespace slices

// Len (Sort interface) returns the length of the NamespaceList
func (al NamespaceList) Len() int {
	return len(al)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the NamespaceList
func (al NamespaceList) Swap(i, j int) {
	al[i], al[j] = al[j], al[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the NamespaceList and returns true if the condition holds, and
// else false.
func (al NamespaceList) Less(i, j int) bool {
	return al[i].Name < al[j].Name
}
