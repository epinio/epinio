// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd_test

import (
	"errors"
	"io"

	"github.com/epinio/epinio/internal/cli/cmd"
	"github.com/epinio/epinio/internal/cli/cmd/cmdfakes"
	"github.com/epinio/epinio/internal/cli/usercmd/usercmdfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	//	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Command 'epinio service'", func() {

	var (
		mockServiceService *cmdfakes.FakeServicesService
		mockAPIClient      *usercmdfakes.FakeAPIClient
		output, outputErr  io.ReadWriter
		args               []string
	)

	BeforeEach(func() {
		mockServiceService = &cmdfakes.FakeServicesService{}
		mockAPIClient = &usercmdfakes.FakeAPIClient{}
		mockServiceService.GetAPIReturns(mockAPIClient)

		args = []string{}
	})

	// TODO: bind, unbind, update, delete, port-forward

	Context("service create", func() {

		When("called with no args", func() {
			It("fails", func() {
				serviceCmd := cmd.NewServiceCreateCmd(mockServiceService)
				_, _, runErr := executeCmd(serviceCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("accepts 2 arg(s), received 0"))
			})
		})

		When("called with more than 2 args", func() {
			It("fails", func() {
				args = append(args, "something", "more", "word")

				serviceCmd := cmd.NewServiceCreateCmd(mockServiceService)
				_, _, runErr := executeCmd(serviceCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("accepts 2 arg(s), received 3"))
			})
		})

		When("the service create fails", func() {
			It("returns an error", func() {
				args = append(args, "myservice", "hey")

				mockServiceService.ServiceCreateStub = func(c, s string, w bool, cv models.ChartValueSettings) error {
					Expect(c).To(Equal("myservice"))
					Expect(s).To(Equal("hey"))
					return errors.New("something bad happened")
				}

				serviceCmd := cmd.NewServiceCreateCmd(mockServiceService)
				_, _, runErr := executeCmd(serviceCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("error creating service: something bad happened"))
			})
		})

		When("the service create succeeds", func() {
			It("returns ok", func() {
				args = append(args, "myservice", "hey")

				mockServiceService.ServiceCreateStub = func(c, s string, w bool, cv models.ChartValueSettings) error {
					Expect(c).To(Equal("myservice"))
					Expect(s).To(Equal("hey"))
					return nil
				}

				serviceCmd := cmd.NewServiceCreateCmd(mockServiceService)
				_, _, runErr := executeCmd(serviceCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
			})
		})
	})

	Context("service list", func() {

		When("called with one or more args", func() {
			It("fails", func() {
				args = append(args, "one")

				serviceCmd := cmd.NewServiceListCmd(mockServiceService, cmd.NewRootConfig())
				_, _, runErr := executeCmd(serviceCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal(`accepts 0 arg(s), received 1`))

				args = append(args, "two")

				_, _, runErr = executeCmd(serviceCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal(`accepts 0 arg(s), received 2`))
			})
		})

		When("the service list fails", func() {
			It("returns an error", func() {
				mockServiceService.ServiceListReturns(errors.New("something bad happened"))

				serviceCmd := cmd.NewServiceListCmd(mockServiceService, cmd.NewRootConfig())
				_, _, runErr := executeCmd(serviceCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("error listing services: something bad happened"))
			})
		})

		When("the service returns ok", func() {
			It("does not return an error", func() {
				mockServiceService.ServiceListReturns(nil)

				serviceCmd := cmd.NewServiceListCmd(mockServiceService, cmd.NewRootConfig())
				_, _, runErr := executeCmd(serviceCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
			})

			It("shows that the default output flag is 'text'", func() {
				mockServiceService.ServiceListReturns(nil)

				rootCfg := cmd.NewRootConfig()
				serviceCmd := cmd.NewServiceListCmd(mockServiceService, rootCfg)
				_, _, runErr := executeCmd(serviceCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
				Expect(rootCfg.Output.Value).To(Equal("text"))
			})
		})

		When("called with the output flag to json", func() {
			It("shows that the output is 'json'", func() {
				args = append(args, "--output", "json")
				mockServiceService.ServiceListReturns(nil)

				rootCfg := cmd.NewRootConfig()
				serviceCmd := cmd.NewServiceListCmd(mockServiceService, rootCfg)
				_, _, runErr := executeCmd(serviceCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
				Expect(rootCfg.Output.Value).To(Equal("json"))
			})
		})
	})
})
