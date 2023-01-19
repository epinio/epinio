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

// Implement the Sort interface for service slices

// Len (Sort interface) returns the length of the ServiceList
func (srl ServiceList) Len() int {
	return len(srl)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the ServiceList
func (srl ServiceList) Swap(i, j int) {
	srl[i], srl[j] = srl[j], srl[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the ServiceList and returns true if the
// condition holds, and else false.
func (srl ServiceList) Less(i, j int) bool {
	// Comparison is done on the namespace names first, and then on the configuration names.
	return (srl[i].Meta.Namespace < srl[j].Meta.Namespace) ||
		((srl[i].Meta.Namespace == srl[j].Meta.Namespace) &&
			(srl[i].Meta.Name < srl[j].Meta.Name))
}
