package helpers

import (
	"io/ioutil"
	"log"
	"path"

	"github.com/rakyll/statik/fs"
	_ "github.com/suse/carrier/cli/statik"
)

// UnEmbedFile creates a file in a temporary directory on disk from a file
// embedded with statik. It returns the path to the created file.
// Caller should make sure the file is deleted after usage (possibly with a defer).
func UnEmbedFile(filePath string) (string, error) {
	statikFS, err := fs.New()
	if err != nil {
		log.Fatal(err)
	}

	file, err := statikFS.Open(path.Join("/", filePath))
	if err != nil {
		return "", err
	}

	tarballContents, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}

	tmpFilePath, err := CreateTmpFile(string(tarballContents))
	if err != nil {
		return "", err
	}

	return tmpFilePath, nil
}
