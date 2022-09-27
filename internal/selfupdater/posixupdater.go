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
	err = validateFileChecksum(tmpFile, checksumFileURL, fmt.Sprintf("epinio-linux-%s", URLArch))
	if err != nil {
		return errors.Wrap(err, "validating file checksum")
	}

	if err := os.Rename(tmpFile, binaryInfo.Path); err != nil {
		return errors.Wrap(err, "moving the temporary file to its final location")
	}

	err = os.Chmod(binaryInfo.Path, binaryInfo.Permissions)
	if err != nil {
		return errors.Wrap(err, "setting the new file permissions")
	}

	return nil
}
