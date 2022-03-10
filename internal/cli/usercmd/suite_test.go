package usercmd_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEpinio(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI usercmd unit test suite")
}
