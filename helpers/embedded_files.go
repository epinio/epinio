package helpers

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/rakyll/statik/fs"
	_ "github.com/suse/carrier/statik"
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

	return Kubectl(fmt.Sprintf("apply --filename %s", yamlPathOnDisk))
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
		return Kubectl(fmt.Sprintf("delete --ignore-not-found=true --wait=false --filename %s", yamlPathOnDisk))
	} else {
		return Kubectl(fmt.Sprintf("delete --filename %s", yamlPathOnDisk))
	}
}
