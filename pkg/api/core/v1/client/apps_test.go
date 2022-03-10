package client_test

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Client Apps unit tests", func() {
	FDescribe("AppLogs", DescribeAppLogs)
	Describe("AppRestart", DescribeAppRestart)
})
