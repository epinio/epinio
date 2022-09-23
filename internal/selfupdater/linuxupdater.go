package selfupdater

import (
	"fmt"
	"os"
	"runtime"

	"github.com/pkg/errors"
)

type LinuxUpdater struct {
}

func (u LinuxUpdater) Update(targetVersion string) error {
	arch := runtime.GOARCH

	URLArch, known := ArchToURL[arch]
	if !known {
		return errors.Errorf("unknown architecture: %s", arch)
	}
	binaryURL := fmt.Sprintf(GithubBinaryURLFormat, targetVersion, "linux", URLArch)

	binaryInfo, err := currentBinaryInfo()
	if err != nil {
		return errors.Wrap(err, "extracting information from the current binary")
	}

	tmpFile, err := downloadBinary(binaryURL, binaryInfo.Dir)
	if err != nil {
		return errors.Wrapf(err, "downloading the binary for version %s", targetVersion)
	}
	defer os.Remove(tmpFile)

	if err := os.Rename(tmpFile, binaryInfo.Path); err != nil {
		return errors.Wrap(err, "moving the temporary file to its final location")
	}

	err = os.Chmod(binaryInfo.Path, binaryInfo.Permissions)
	if err != nil {
		return errors.Wrap(err, "setting the new file permissions")
	}

	return nil
}
