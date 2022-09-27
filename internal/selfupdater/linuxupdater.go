package selfupdater

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strings"

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

func validateFileChecksum(filePath, checksumFileURL, fileNamePattern string) error {
	tmpFileChecksum, err := calculateChecksum(filePath)
	if err != nil {
		return errors.Wrap(err, "calculating binary file checksum")
	}

	tmpDir, err := ioutil.TempDir("", "epinio")
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
