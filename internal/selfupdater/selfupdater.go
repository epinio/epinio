// Package selfupdater is used to replace the current running binary, with
// a given version. It is used to sync the cli to the server version.
package selfupdater

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	progressbar "github.com/schollz/progressbar/v3"
)

const GithubBinaryURLFormat = "https://github.com/epinio/epinio/releases/download/%s/epinio-%s-%s"

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

type Updater interface {
	Update(string) error
}

// downloadBinary downloads a remote file to the specified directory, using
// a "random" name. It returns the new file path and/or and error if one occurs.
func downloadBinary(remoteURL, dir string) (string, error) {
	tmpFile, err := ioutil.TempFile(dir, "epinio")
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

	fmt.Printf("Downloading file %s\n", remoteURL)
	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"Progress",
	)

	_, err = io.Copy(io.MultiWriter(tmpFile, bar), resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "downloading the remote file")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", errors.Errorf("unexpected status code: %d", resp.StatusCode)
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
