package cmd_test

import (
	"errors"
	"io"

	"github.com/epinio/epinio/internal/cli/cmd"
	"github.com/epinio/epinio/internal/cli/cmd/cmdfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Command 'epinio app chart push'", func() {

	var (
		mockChartService  *cmdfakes.FakeAppchartsService
		output, outputErr io.ReadWriter
	)

	BeforeEach(func() {
		mockChartService = &cmdfakes.FakeAppchartsService{}
	})

	When("called with too few args", func() {
		It("fails", func() {
			pushCmd := cmd.NewAppChartPushCmd(mockChartService)
			_, _, runErr := executeCmd(pushCmd, []string{"mychart"}, output, outputErr)
			Expect(runErr).To(HaveOccurred())
			Expect(runErr.Error()).To(Equal("accepts 2 arg(s), received 1"))
			Expect(mockChartService.ChartPushCallCount()).To(Equal(0))
		})
	})

	When("called with too many args", func() {
		It("fails", func() {
			pushCmd := cmd.NewAppChartPushCmd(mockChartService)
			_, _, runErr := executeCmd(pushCmd, []string{"mychart", "a.tgz", "b.tgz"}, output, outputErr)
			Expect(runErr).To(HaveOccurred())
			Expect(runErr.Error()).To(Equal("accepts 2 arg(s), received 3"))
			Expect(mockChartService.ChartPushCallCount()).To(Equal(0))
		})
	})

	When("the push fails", func() {
		It("returns an error", func() {
			mockChartService.ChartPushReturns(errors.New("something bad happened"))

			pushCmd := cmd.NewAppChartPushCmd(mockChartService)
			_, _, runErr := executeCmd(pushCmd, []string{"mychart", "mychart-0.1.0.tgz"}, output, outputErr)
			Expect(runErr).To(HaveOccurred())
			Expect(runErr.Error()).To(Equal("error pushing app chart: something bad happened"))
		})
	})

	When("the push succeeds", func() {
		It("passes the arguments and descriptions on", func() {
			pushCmd := cmd.NewAppChartPushCmd(mockChartService)
			_, _, runErr := executeCmd(pushCmd, []string{
				"mychart", "mychart-0.1.0.tgz",
				"--description", "long text",
				"--short-description", "short text",
			}, output, outputErr)
			Expect(runErr).ToNot(HaveOccurred())

			Expect(mockChartService.ChartPushCallCount()).To(Equal(1))
			_, name, archive, description, shortDescription := mockChartService.ChartPushArgsForCall(0)
			Expect(name).To(Equal("mychart"))
			Expect(archive).To(Equal("mychart-0.1.0.tgz"))
			Expect(description).To(Equal("long text"))
			Expect(shortDescription).To(Equal("short text"))
		})

		It("leaves the descriptions empty by default", func() {
			pushCmd := cmd.NewAppChartPushCmd(mockChartService)
			_, _, runErr := executeCmd(pushCmd, []string{"mychart", "mychart-0.1.0.tgz"}, output, outputErr)
			Expect(runErr).ToNot(HaveOccurred())

			_, _, _, description, shortDescription := mockChartService.ChartPushArgsForCall(0)
			Expect(description).To(BeEmpty())
			Expect(shortDescription).To(BeEmpty())
		})
	})

	It("is registered as subcommand of 'epinio app chart'", func() {
		chartCmd := cmd.NewAppChartCmd(mockChartService)

		found := false
		for _, sub := range chartCmd.Commands() {
			if sub.Name() == "push" {
				found = true
			}
		}
		Expect(found).To(BeTrue())
	})

})
