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

package mask

// MaskValue masks a sensitive value for display purposes.
// Returns "****" for any non-empty value to prevent exposure of secrets.
func MaskValue(value string) string {
	if value == "" {
		return ""
	}
	return "****"
}

// MaskMap masks all values in a map[string]string, returning a new map
// with the same keys but masked values.
func MaskMap(data map[string]string) map[string]string {
	if data == nil {
		return nil
	}
	masked := make(map[string]string, len(data))
	for k, v := range data {
		masked[k] = MaskValue(v)
	}
	return masked
}

