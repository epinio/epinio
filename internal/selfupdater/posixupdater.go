package selfupdater

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/pkg/errors"
)

type PosixUpdater struct {
}

func (u PosixUpdater) Update(targetVersion string) error {
	currentArch := runtime.GOARCH
	currentOS := runtime.GOOS

	URLArch, known := ArchToURL[currentArch]
	if !known {
		return errors.Errorf("unknown architecture: %s", currentArch)
	}
	binaryURL := fmt.Sprintf(GithubBinaryURLFormat, targetVersion, currentOS, URLArch)

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
	err = validateFileChecksum(tmpFile, checksumFileURL, fmt.Sprintf("epinio-%s-%s", currentOS, URLArch))
	if err != nil {
		return errors.Wrap(err, "validating file checksum")
	}

	// https://github.com/flavio/kuberlr/blob/b4d047a69efec991a27133b5362443f48a9a1225/internal/downloader/download.go#L196
	if err := os.Rename(tmpFile, binaryInfo.Path); err != nil {
		linkErr, ok := err.(*os.LinkError)
		if ok {
			fmt.Fprintf(os.Stderr, "Cross-device error trying to rename a file: %s -- will do a full copy\n", linkErr)
			var tempInput []byte
			tempInput, err = os.ReadFile(tmpFile)
			if err != nil {
				return errors.Wrapf(err, "Error reading temporary file %s", tmpFile)
			}
			err = os.WriteFile(binaryInfo.Path, tempInput, binaryInfo.Permissions)
			if err != nil {
				return errors.Wrap(err, "copying new binary to its destination")
			}
		} else {
			return errors.Wrap(err, "moving the temporary file to its final location")
		}
	}

	err = os.Chmod(binaryInfo.Path, binaryInfo.Permissions)
	if err != nil {
		return errors.Wrap(err, "setting the new file permissions")
	}

	return nil
}
