package appchart_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAppChart(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AppChart API unit test suite")
}
