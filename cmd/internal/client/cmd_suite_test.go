package client_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCarrier(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Carrier Suite")
}
