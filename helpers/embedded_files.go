package helpers

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path"

	// We need the imports for the side effects, i.e. the `init()`
	// functions they run to register their filesystems. As none
	// of the public symbols are directly used an non-blank import
	// would be elided by go format.
	_ "github.com/epinio/epinio/assets/statik"
	"github.com/rakyll/statik/fs"
)

// ExtractFile creates a file in a temporary directory on disk from a file
// embedded with statik. It returns the path to the created file.
// Caller should make sure the file is deleted after usage (possibly with a defer).
func ExtractFile(filePath string) (string, error) {
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

// KubectlApplyEmbeddedYaml un-embeds the given yaml file and calls `kubectl apply`
// on it. It returns the command output and an error (if there is one)
func KubectlApplyEmbeddedYaml(yamlPath string) (string, error) {
	yamlPathOnDisk, err := ExtractFile(yamlPath)
	if err != nil {
		return "", errors.New("Failed to extract embedded file: " + yamlPath + " - " + err.Error())
	}
	defer os.Remove(yamlPathOnDisk)

	return Kubectl("apply", "--filename", yamlPathOnDisk)
}

// KubectlDeleteEmbeddedYaml un-embeds the given yaml file and calls `kubectl delete`
// on it. It returns the command output and an error (if there is one)
func KubectlDeleteEmbeddedYaml(yamlPath string, ignoreMissing bool) (string, error) {
	yamlPathOnDisk, err := ExtractFile(yamlPath)
	if err != nil {
		return "", errors.New("Failed to extract embedded file: " + yamlPath + " - " + err.Error())
	}
	defer os.Remove(yamlPathOnDisk)

	if ignoreMissing {
		return Kubectl("delete",
			"--ignore-not-found=true",
			"--wait=false",
			"--filename", yamlPathOnDisk)
	}
	return Kubectl("delete", "--filename", yamlPathOnDisk)
}
