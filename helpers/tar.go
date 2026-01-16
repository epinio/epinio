// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package helpers

import (
	"context"
	"os"
	"path"
	"path/filepath"

	"github.com/mholt/archives"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

const blobName = "blob.tar"

const epinioIgnoreFile = ".epinioignore"

var excludedGitFiles = []string{
	".git",
	".gitignore",
	".gitmodules",
	".gitconfig",
	".git-credentials",
}

// Tar creates a tarball of a directory, excluding files based on ignore patterns.
// The ignore patterns can come from:
//   - .epinioignore file in the directory
//   - manifestPatterns parameter (from epinio.yaml)
//
// Patterns from both sources are merged, with .epinioignore patterns processed after
// manifest patterns (so they can override if needed).
func Tar(dir string, manifestPatterns []string) (string, string, error) {
	ctx := context.TODO()

	// Normalize the directory path first (before loading .epinioignore)
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", "", errors.Wrap(err, "cannot get absolute path")
	}

	// Load ignore patterns from .epinioignore and merge with manifest patterns
	ignoreMatcher, err := LoadIgnoreMatcher(dir, manifestPatterns)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to load ignore patterns")
	}

	sources := make(map[string]string)

	// Recursively walk the directory tree
	err = filepath.Walk(dir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			// If we can't read a file/directory, skip it but continue
			return nil
		}

		// Get relative path for checking ignore patterns
		relPath, err := filepath.Rel(dir, filePath)
		if err != nil {
			return nil // Skip if we can't get relative path
		}

		// Always ignore git config files
		baseName := filepath.Base(relPath)
		if slices.Contains(excludedGitFiles, baseName) {
			if info.IsDir() {
				return filepath.SkipDir // Skip the entire .git directory
			}
			return nil // Skip this file
		}

		// Check if this path should be ignored based on .epinioignore
		if ignoreMatcher.ShouldIgnore(dir, filePath, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir // Skip the entire ignored directory
			}
			return nil // Skip this ignored file
		}

		// Also ignore .epinioignore file itself (don't include it in the tarball)
		if baseName == epinioIgnoreFile {
			return nil
		}

		// Only add files to sources, not directories
		// The archives library will create directory structure automatically from file paths
		// Adding directories would cause it to recursively include all files, bypassing ignore logic
		if info.IsDir() {
			// Skip directories - they'll be created automatically when we add files
			return nil
		}

		// Add this file to sources
		// The key is the full path, the value is the relative path in the tarball
		sources[filePath] = relPath

		return nil
	})

	if err != nil {
		return "", "", errors.Wrap(err, "error walking directory tree")
	}
	
	filesFromDisk, filesFromDiskError := archives.FilesFromDisk(
		ctx,
		nil,
		sources,
	)
	if filesFromDiskError != nil {
		return "", "", errors.Wrap(filesFromDiskError, "can't create files from disk for tarball")
	}

	// create a tmpDir - tarball dir and POST
	tmpDir, tmpDirError := os.MkdirTemp("", "epinio-app")
	if tmpDirError != nil {
		return "", "", errors.Wrap(tmpDirError, "can't create output directory")
	}

	tarballName := path.Join(tmpDir, blobName)

	// Open file reference to the tarball file
	outFile, outFileError := os.Create(tarballName)
	if outFileError != nil {
		return tmpDir, "", errors.Wrap(outFileError, "can't create output file")
	}

	defer func () {
		if closeErr := outFile.Close(); closeErr != nil {
			_ = errors.Wrap(closeErr, "can't close output file")
		}
	}()

	// For this case, we just care about the Archival option, but archives supplies
	// more attributes to set here: https://pkg.go.dev/github.com/mholt/archives#CompressedArchive
	// if we need to get more agessive with the compression.
	format := archives.CompressedArchive{
		Archival: archives.Tar{},
	}
	
	writerError := format.Archive(ctx, outFile, filesFromDisk)
	if writerError != nil {
		return tmpDir, "", errors.Wrap(writerError, "can't create tar writer")
	}

	return tmpDir, tarballName, nil
}
