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

package termui

// WithTable prints a new table
func (u *Message) WithTable(headers ...string) *Message {
	u.tableHeaders = append(u.tableHeaders, headers)
	u.tableData = append(u.tableData, [][]string{})
	return u
}

// WithTableRow adds a row in the latest table
func (u *Message) WithTableRow(values ...string) *Message {
	if len(u.tableHeaders) < 1 {
		return u.WithTable(make([]string, len(values))...).WithTableRow(values...)
	}

	u.tableData[len(u.tableData)-1] = append(u.tableData[len(u.tableData)-1], values)

	return u
}
