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

package selfupdater

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
)

type WindowsUpdater struct {
}

func (u WindowsUpdater) Update(targetVersion string) error {
	currentArch := runtime.GOARCH
	currentOS := "windows"

	URLArch, known := ArchToURL[currentArch]
	if !known {
		return errors.Errorf("unknown architecture: %s", currentArch)
	}
	binaryURL := fmt.Sprintf(GithubBinaryURLFormat, targetVersion, currentOS, URLArch) + ".zip"

	binaryInfo, err := currentBinaryInfo()
	if err != nil {
		return errors.Wrap(err, "extracting information from the current binary")
	}

	// Download to the same directory to avoid errors when replacing the binary.
	tmpFile, err := downloadFile(binaryURL, binaryInfo.Dir)
	if err != nil {
		return errors.Wrapf(err, "downloading the binary for version %s", targetVersion)
	}
	defer os.Remove(tmpFile)

	checksumFileURL := fmt.Sprintf(GithubChecksumURLFormat, targetVersion, strings.TrimPrefix(targetVersion, "v"))
	err = validateFileChecksum(tmpFile, checksumFileURL, fmt.Sprintf("epinio-windows-%s.zip", URLArch))
	if err != nil {
		return errors.Wrap(err, "validating file checksum")
	}

	tmpDir, err := os.MkdirTemp("", "epinio")
	if err != nil {
		return errors.Wrap(err, "creating temporary directory")
	}
	defer os.RemoveAll(tmpDir)
	err = unzip(tmpFile, tmpDir)
	if err != nil {
		return errors.Wrap(err, "extracting zip file")
	}

	// Move the current binary to "epinio.bak" in the tmp directory.
	// This will allow us to copy the new binary to the current path.
	// Ideas from here:
	// https://github.com/restic/restic/issues/2248#issuecomment-903872651
	backupFilePath := filepath.Join(tmpDir, "epinio.bak")
	if err := os.Rename(binaryInfo.Path, backupFilePath); err != nil {
		return errors.Wrap(err, "moving the current binary to a backup")
	}

	if err := os.Rename(filepath.Join(tmpDir, "epinio.exe"), binaryInfo.Path); err != nil {
		return errors.Wrap(err, "moving the new binary to its final location")
	}

	err = os.Chmod(binaryInfo.Path, binaryInfo.Permissions)
	if err != nil {
		return errors.Wrap(err, "setting the new file permissions")
	}

	return nil
}

// Code copied from: https://stackoverflow.com/a/58192644
func unzip(src, dest string) error {
	dest = filepath.Clean(dest) + string(os.PathSeparator)

	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	err = os.MkdirAll(dest, 0755)
	if err != nil {
		return errors.Wrap(err, "creating the zip extracting destination directory")
	}

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		// Check for ZipSlip: https://snyk.io/research/zip-slip-vulnerability
		destpath := filepath.Join(dest, f.Name) // nolint:gosec // Following code mitigates the issue
		if !strings.HasPrefix(destpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("%s: illegal file path", f.Name)
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		if f.FileInfo().IsDir() {
			err := os.MkdirAll(destpath, f.Mode())
			if err != nil {
				return errors.Wrap(err, "creating directory")
			}
		} else {
			err := os.MkdirAll(filepath.Dir(destpath), f.Mode())
			if err != nil {
				return errors.Wrap(err, "creating directory")
			}
			f, err := os.OpenFile(destpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			for {
				_, err := io.CopyN(f, rc, 1024)
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					return err
				}
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}
