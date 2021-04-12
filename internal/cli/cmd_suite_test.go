package cli_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestEpinio(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Epinio Suite")
}
