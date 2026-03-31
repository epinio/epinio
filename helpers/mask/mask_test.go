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

import (
	"testing"
)

func TestMaskValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string returns empty",
			input:    "",
			expected: "",
		},
		{
			name:     "short password is masked",
			input:    "abc",
			expected: "****",
		},
		{
			name:     "long password is masked",
			input:    "supersecretpassword123!@#",
			expected: "****",
		},
		{
			name:     "single character is masked",
			input:    "x",
			expected: "****",
		},
		{
			name:     "whitespace only is masked",
			input:    "   ",
			expected: "****",
		},
		{
			name:     "special characters are masked",
			input:    "p@$$w0rd!",
			expected: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskValue(tt.input)
			if result != tt.expected {
				t.Errorf("MaskValue(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "nil map returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map returns empty map",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "single entry is masked",
			input: map[string]string{
				"password": "secret123",
			},
			expected: map[string]string{
				"password": "****",
			},
		},
		{
			name: "multiple entries are masked",
			input: map[string]string{
				"username":     "admin",
				"password":     "secret123",
				"api_key":      "key-abc-123",
				"empty_value":  "",
				"database_url": "postgresql://user:pass@host:5432/db",
			},
			expected: map[string]string{
				"username":     "****",
				"password":     "****",
				"api_key":      "****",
				"empty_value":  "",
				"database_url": "****",
			},
		},
		{
			name: "keys are preserved",
			input: map[string]string{
				"key-with-dashes":     "value1",
				"key_with_underscores": "value2",
				"UPPERCASE_KEY":        "value3",
			},
			expected: map[string]string{
				"key-with-dashes":     "****",
				"key_with_underscores": "****",
				"UPPERCASE_KEY":        "****",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskMap(tt.input)

			// Check nil case
			if tt.expected == nil {
				if result != nil {
					t.Errorf("MaskMap(nil) = %v, want nil", result)
				}
				return
			}

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("MaskMap() returned map with %d entries, want %d", len(result), len(tt.expected))
			}

			// Check each key-value pair
			for key, expectedValue := range tt.expected {
				actualValue, ok := result[key]
				if !ok {
					t.Errorf("MaskMap() missing key %q", key)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("MaskMap()[%q] = %q, want %q", key, actualValue, expectedValue)
				}
			}
		})
	}
}

func TestMaskMapDoesNotModifyOriginal(t *testing.T) {
	original := map[string]string{
		"password": "secret123",
		"api_key":  "key-abc-123",
	}

	// Store original values
	originalPassword := original["password"]
	originalApiKey := original["api_key"]

	// Call MaskMap
	_ = MaskMap(original)

	// Verify original is unchanged
	if original["password"] != originalPassword {
		t.Errorf("MaskMap modified original map: password changed from %q to %q", originalPassword, original["password"])
	}
	if original["api_key"] != originalApiKey {
		t.Errorf("MaskMap modified original map: api_key changed from %q to %q", originalApiKey, original["api_key"])
	}
}

