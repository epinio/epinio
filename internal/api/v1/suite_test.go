package v1_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEpinio(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Epinio V1 Suite")
}
