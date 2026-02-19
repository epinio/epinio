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

import (
	"context"
	"errors"
	"testing"
)

func TestValidateBuilderImage(t *testing.T) {
	tests := []struct {
		name   string
		image  string
		valid  bool
		hasMsg bool
	}{
		{"valid jammy-full with version tag", "paketobuildpacks/builder-jammy-full:0.3.495", true, false},
		{"valid full", "paketobuildpacks/builder:full", true, false},
		{"valid with latest", "paketobuildpacks/builder-jammy-full:latest", true, false},
		{"invalid wildcard in tag", "paketobuildpacks/builder:*", false, true},
		{"invalid empty", "", false, true},
		{"invalid parse", "not-a-valid-reference:", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateBuilderImage(tt.image)
			if got.Valid != tt.valid {
				t.Errorf("ValidateBuilderImage(%q).Valid = %v, want %v", tt.image, got.Valid, tt.valid)
			}
			if tt.hasMsg && got.Message == "" {
				t.Errorf("ValidateBuilderImage(%q) expected non-empty Message", tt.image)
			}
			if !tt.valid && got.Valid && got.Message != "" {
				t.Errorf("ValidateBuilderImage(%q) valid but has Message: %s", tt.image, got.Message)
			}
		})
	}
}

func TestValidateBuilderImageWithContextRegistryChecks(t *testing.T) {
	previous := imageExistsInRegistryFn
	t.Cleanup(func() { imageExistsInRegistryFn = previous })

	tests := []struct {
		name       string
		exists     bool
		err        error
		wantValid  bool
		wantSubstr string
	}{
		{
			name:      "registry reports image exists",
			exists:    true,
			wantValid: true,
		},
		{
			name:       "registry reports image missing",
			exists:     false,
			wantValid:  false,
			wantSubstr: "image not found in registry",
		},
		{
			name:       "registry check fails",
			err:        errors.New("timeout"),
			wantValid:  false,
			wantSubstr: "unable to verify image in registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imageExistsInRegistryFn = func(context.Context, string) (bool, error) {
				return tt.exists, tt.err
			}

			got := ValidateBuilderImageWithContext(context.Background(), "paketobuildpacks/builder:full", true)
			if got.Valid != tt.wantValid {
				t.Fatalf("ValidateBuilderImageWithContext().Valid = %v, want %v", got.Valid, tt.wantValid)
			}
			if tt.wantSubstr != "" && got.Message != tt.wantSubstr {
				t.Fatalf("ValidateBuilderImageWithContext().Message = %q, want %q", got.Message, tt.wantSubstr)
			}
		})
	}
}
