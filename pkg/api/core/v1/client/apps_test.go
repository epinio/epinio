package client_test

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Client Apps unit tests", func() {
	Describe("AppLogs", DescribeAppLogs)
	Describe("AppRestart", DescribeAppRestart)
})
