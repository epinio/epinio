package helpers_test

import (
	"os"
	"path"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Helpers Suite")
}

// assetPath returns the full path to a file in "fixtures" directory
func assetPath(fileName string) string {
	currentPath, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}

	return path.Join(currentPath, "..", "assets", "tests", fileName)
}
