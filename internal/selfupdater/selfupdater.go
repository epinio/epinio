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

// Package selfupdater is used to replace the current running binary, with
// a given version. It is used to sync the cli to the server version.
package selfupdater

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
	progressbar "github.com/schollz/progressbar/v3"
)

const (
	GithubBinaryURLFormat   = "https://github.com/epinio/epinio/releases/download/%s/epinio-%s-%s"
	GithubChecksumURLFormat = "https://github.com/epinio/epinio/releases/download/%s/epinio_%s_checksums.txt"
)

type BinaryInfo struct {
	Path        string
	Dir         string
	Permissions fs.FileMode
}

// ArchToURL is a map from GOARCH to the arch as it's set in the url of the github assets.
// E.g. the binary for amd64 has a suffix "x86_64" in the assets here:
// https://github.com/epinio/epinio/releases/tag/v1.2.0
// NOTE: If we change how we name the assets, this code will break.
var ArchToURL = map[string]string{
	"arm64": "arm64",
	"s390x": "s390x",
	"arm":   "armv7",
	"amd64": "x86_64",
}

//counterfeiter:generate -header ../../LICENSE_HEADER . Updater
type Updater interface {
	Update(string) error
}

// downloadFile downloads a remote file to the specified directory, using
// a "random" name. It returns the new file path and/or and error if one occurs.
func downloadFile(remoteURL, dir string) (string, error) {
	tmpFile, err := os.CreateTemp(dir, "epinio")
	if err != nil {
		return "", errors.Wrap(err, "creating a temporary file")
	}
	defer tmpFile.Close()

	req, err := http.NewRequest("GET", remoteURL, nil)
	if err != nil {
		return "", errors.Wrap(err, "constructing a request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "making the request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	fmt.Printf("Downloading file %s\n", remoteURL)
	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"Progress",
	)

	_, err = io.Copy(io.MultiWriter(tmpFile, bar), resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "downloading the remote file")
	}

	return tmpFile.Name(), nil
}

// currentBinaryInfo returns a BinaryInfo with information about the currently
// running binary or an error if one occurs.
func currentBinaryInfo() (BinaryInfo, error) {
	var err error
	result := BinaryInfo{}

	result.Path, err = os.Executable()
	if err != nil {
		return result, errors.Wrap(err, "getting the current executable path")
	}
	result.Dir = filepath.Dir(result.Path)

	info, err := os.Stat(result.Path)
	if err != nil {
		return result, errors.Wrap(err, "getting the current executable permissions")
	}
	result.Permissions = info.Mode()

	return result, nil
}

func validateFileChecksum(filePath, checksumFileURL, fileNamePattern string) error {
	tmpFileChecksum, err := calculateChecksum(filePath)
	if err != nil {
		return errors.Wrap(err, "calculating binary file checksum")
	}

	tmpDir, err := os.MkdirTemp("", "epinio")
	if err != nil {
		return errors.Wrap(err, "creating temporary directory")
	}
	defer os.RemoveAll(tmpDir)

	tmpChecksumFile, err := downloadFile(checksumFileURL, tmpDir)
	if err != nil {
		return errors.Wrap(err, "downloading checksum file")
	}

	checksumFileContents, err := os.ReadFile(tmpChecksumFile)
	if err != nil {
		return errors.Wrap(err, "reading the checksum file")
	}

	re := regexp.MustCompile(fmt.Sprintf(`([a-z,0-9]+)\s+%s`, fileNamePattern))
	matches := re.FindStringSubmatch(string(checksumFileContents))
	if len(matches) < 2 {
		return errors.New("couldn't find a checksum for the given file")
	}

	if matches[1] != tmpFileChecksum {
		return errors.New("file checksum invalid")
	}

	return nil
}

func calculateChecksum(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", errors.Wrapf(err, "opening file %s", filePath)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", errors.Wrap(err, "calculating checksum")
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
