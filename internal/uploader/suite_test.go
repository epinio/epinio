package uploader_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUploader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Epinio uploader suite")
}
