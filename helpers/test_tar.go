// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/epinio/epinio/helpers"
)

// This is a helper tool to test what files would be included in a tarball
// Usage: go run test_tar.go <directory>
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <directory>\n", os.Args[0])
		os.Exit(1)
	}

	dir := os.Args[1]
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Directory does not exist: %s\n", dir)
		os.Exit(1)
	}

	// Load ignore matcher (no manifest patterns for this test)
	matcher, err := helpers.LoadIgnoreMatcher(dir, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading ignore matcher: %v\n", err)
		os.Exit(1)
	}

	// Walk directory and list what would be included
	var included []string
	var ignored []string

	err = filepath.Walk(dir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't read
		}

		relPath, err := filepath.Rel(dir, filePath)
		if err != nil {
			return nil
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		// Check if ignored
		if matcher.ShouldIgnore(dir, filePath, info.IsDir()) {
			ignored = append(ignored, relPath)
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Also skip .epinioignore file itself
		if filepath.Base(relPath) == ".epinioignore" {
			return nil
		}

		included = append(included, relPath)
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
		os.Exit(1)
	}

	// Sort for readability
	sort.Strings(included)
	sort.Strings(ignored)

	fmt.Println("=== Files that WOULD BE INCLUDED in tarball ===")
	if len(included) == 0 {
		fmt.Println("(none)")
	} else {
		for _, f := range included {
			fmt.Printf("  + %s\n", f)
		}
	}

	fmt.Println("\n=== Files that WOULD BE IGNORED (not in tarball) ===")
	if len(ignored) == 0 {
		fmt.Println("(none)")
	} else {
		for _, f := range ignored {
			fmt.Printf("  - %s\n", f)
		}
	}

	fmt.Printf("\nTotal: %d included, %d ignored\n", len(included), len(ignored))
}

