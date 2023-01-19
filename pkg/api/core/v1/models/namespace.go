// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package models

// Namespace has all the namespace properties, i.e. name, app names, and configuration names
// It is used in the CLI and API responses.
type Namespace struct {
	Meta           MetaLite `json:"meta,omitempty"`
	Apps           []string `json:"apps,omitempty"`
	Configurations []string `json:"configurations,omitempty"`
}

// NamespaceList is a collection of namespaces
type NamespaceList []Namespace

// Implement the Sort interface for namespace slices
// Namespaces are sorted by their names

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
	return al[i].Meta.Name < al[j].Meta.Name
}
