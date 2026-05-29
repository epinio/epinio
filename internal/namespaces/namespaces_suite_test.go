package namespaces_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNamespaces(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Namespaces unit test suite")
}
