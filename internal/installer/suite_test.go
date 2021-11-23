package installer_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"path"
)

func TestEpinio(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Epinio Install Manifest Suite")
}

func assetPath(asset string) string {
	return path.Join("../../assets/tests", asset)
}
