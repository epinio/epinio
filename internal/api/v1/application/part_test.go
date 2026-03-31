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

package application

import "testing"

// expectedPartNames is the canonical set of part names for GET .../part/:part.
// Keep in sync with validPartNames in part.go.
var expectedPartNames = []string{"manifest", "values", "chart", "image", "archive"}

func TestValidPartNamesContract(t *testing.T) {
	t.Run("has expected length", func(t *testing.T) {
		if got := len(validPartNames); got != len(expectedPartNames) {
			t.Errorf("validPartNames length = %d, want %d", got, len(expectedPartNames))
		}
	})

	t.Run("includes archive", func(t *testing.T) {
		var found bool
		for _, p := range validPartNames {
			if p == "archive" {
				found = true
				break
			}
		}
		if !found {
			t.Error("validPartNames must include 'archive'")
		}
	})

	t.Run("equals expected set", func(t *testing.T) {
		if len(validPartNames) != len(expectedPartNames) {
			t.Skip("length mismatch already reported")
		}
		seen := make(map[string]bool)
		for _, p := range validPartNames {
			if seen[p] {
				t.Errorf("duplicate part name: %q", p)
			}
			seen[p] = true
		}
		for _, want := range expectedPartNames {
			if !seen[want] {
				t.Errorf("validPartNames missing %q", want)
			}
		}
	})
}

func TestIsValidPartName(t *testing.T) {
	t.Run("archive is valid", func(t *testing.T) {
		if !isValidPartName("archive") {
			t.Error("expected 'archive' to be a valid part name")
		}
	})

	t.Run("all valid part names accepted", func(t *testing.T) {
		for _, part := range validPartNames {
			if !isValidPartName(part) {
				t.Errorf("expected %q to be valid", part)
			}
		}
	})

	t.Run("invalid part name rejected", func(t *testing.T) {
		invalid := []string{
			"", "invalid", "ARCHIVE", "chart ", " image", "values\n",
			"arch", "archiv", "archives", "manifesto", "chart\t",
			"Manifest", "VALUES", "Chart", "Image",
		}
		for _, part := range invalid {
			if isValidPartName(part) {
				t.Errorf("expected %q to be invalid", part)
			}
		}
	})

	t.Run("only exact match accepted", func(t *testing.T) {
		// Substrings or supersets of valid names must be rejected
		for _, valid := range validPartNames {
			if len(valid) > 1 && isValidPartName(valid[:1]) {
				t.Errorf("prefix %q of %q should be invalid", valid[:1], valid)
			}
			withSuffix := valid + "x"
			if isValidPartName(withSuffix) {
				t.Errorf("expected %q to be invalid", withSuffix)
			}
		}
	})
}

func TestValidPartNamesNoDuplicates(t *testing.T) {
	seen := make(map[string]bool)
	for _, p := range validPartNames {
		if seen[p] {
			t.Errorf("duplicate in validPartNames: %q", p)
		}
		seen[p] = true
	}
}

func TestExpectedPartNamesUsed(t *testing.T) {
	// Ensure test's expectedPartNames is not stale (same count and includes archive)
	if len(expectedPartNames) != len(validPartNames) {
		t.Errorf("expectedPartNames in test file out of sync with validPartNames (len %d vs %d)",
			len(expectedPartNames), len(validPartNames))
	}
	var hasArchive bool
	for _, p := range expectedPartNames {
		if p == "archive" {
			hasArchive = true
			break
		}
	}
	if !hasArchive {
		t.Error("expectedPartNames must include 'archive'")
	}
}
