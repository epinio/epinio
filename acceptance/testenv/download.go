package testenv

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/epinio/epinio/helpers"
	"github.com/pkg/errors"
)

// CompressionType is used by DownloadDescriptor, it's the compression of the
// downloaded file
type CompressionType int

const (
	None CompressionType = iota
	TarGZ
)

// DownloadDescriptor describes what to download and where to unpack it
type DownloadDescriptor struct {
	URL         string
	Compression CompressionType
	Destination string
}

const dlFilename = "download"

// Download shells out to wget and unpacks the result
func Download(d DownloadDescriptor) error {
	var (
		tmpDir string
		err    error
	)

	if tmpDir, err = ioutil.TempDir("", "epinio-dl"); err != nil {
		return err
	}

	defer func() {
		os.RemoveAll(tmpDir)
	}()

	if out, err := helpers.RunProc(tmpDir, false, "wget", d.URL, "-O", dlFilename); err != nil {
		return errors.Wrap(err, out)
	}

	switch c := d.Compression; c {
	case None:
		return os.Rename(path.Join(tmpDir, dlFilename), d.Destination)
	case TarGZ:
		if out, err := helpers.RunProc(tmpDir, false, "tar", "xvf", dlFilename, "-C", d.Destination, "--strip-components", "1"); err != nil {
			return errors.Wrap(err, out)
		}
	default:
		return errors.New("cannot handle compression type")
	}

	return nil
}
