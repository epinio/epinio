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

	//	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Command 'epinio configuration'", func() {

	var (
		mockConfigurationService *cmdfakes.FakeConfigurationService
		mockAPIClient            *usercmdfakes.FakeAPIClient
		output, outputErr        io.ReadWriter
		args                     []string
	)

	BeforeEach(func() {
		mockConfigurationService = &cmdfakes.FakeConfigurationService{}
		mockAPIClient = &usercmdfakes.FakeAPIClient{}
		mockConfigurationService.GetAPIReturns(mockAPIClient)

		args = []string{}
	})

	// TODO: bind, unbind, update, delete

	Context("configuration create", func() {

		When("called with no args", func() {
			It("fails", func() {
				configurationCmd := cmd.NewConfigurationCreateCmd(mockConfigurationService)
				_, _, runErr := executeCmd(configurationCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("Not enough arguments, expected name"))
			})
		})

		When("called with multiple args, last key has no value", func() {
			It("fails", func() {
				args = append(args, "something", "more")

				configurationCmd := cmd.NewConfigurationCreateCmd(mockConfigurationService)
				_, _, runErr := executeCmd(configurationCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("Last Key has no value"))
			})
		})

		When("the configuration create fails", func() {
			It("returns an error", func() {
				args = append(args, "myconfiguration")

				mockConfigurationService.CreateConfigurationStub = func(s string, kv []string) error {
					Expect(s).To(Equal("myconfiguration"))
					return errors.New("something bad happened")
				}

				configurationCmd := cmd.NewConfigurationCreateCmd(mockConfigurationService)
				_, _, runErr := executeCmd(configurationCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("error creating configuration: something bad happened"))
			})
		})

		When("the configuration create succeeds", func() {
			It("returns ok", func() {
				args = append(args, "myconfiguration", "hey", "you")

				mockConfigurationService.CreateConfigurationStub = func(s string, kv []string) error {
					Expect(s).To(Equal("myconfiguration"))
					return nil
				}

				configurationCmd := cmd.NewConfigurationCreateCmd(mockConfigurationService)
				_, _, runErr := executeCmd(configurationCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
			})
		})
	})

	Context("configuration list", func() {

		When("called with one or more args", func() {
			It("fails", func() {
				args = append(args, "one")

				configurationCmd := cmd.NewConfigurationListCmd(mockConfigurationService, cmd.NewRootConfig())
				_, _, runErr := executeCmd(configurationCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal(`unknown command "one" for "list"`))

				args = append(args, "two")

				_, _, runErr = executeCmd(configurationCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal(`unknown command "one" for "list"`))
			})
		})

		When("the configuration list fails", func() {
			It("returns an error", func() {
				mockConfigurationService.ConfigurationsReturns(errors.New("something bad happened"))

				configurationCmd := cmd.NewConfigurationListCmd(mockConfigurationService, cmd.NewRootConfig())
				_, _, runErr := executeCmd(configurationCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("error listing configurations: something bad happened"))
			})
		})

		When("the configuration returns ok", func() {
			It("does not return an error", func() {
				mockConfigurationService.ConfigurationsReturns(nil)

				configurationCmd := cmd.NewConfigurationListCmd(mockConfigurationService, cmd.NewRootConfig())
				_, _, runErr := executeCmd(configurationCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
			})

			It("shows that the default output flag is 'text'", func() {
				mockConfigurationService.ConfigurationsReturns(nil)

				rootCfg := cmd.NewRootConfig()
				configurationCmd := cmd.NewConfigurationListCmd(mockConfigurationService, rootCfg)
				_, _, runErr := executeCmd(configurationCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
				Expect(rootCfg.Output.Value).To(Equal("text"))
			})
		})

		When("called with the output flag to json", func() {
			It("shows that the output is 'json'", func() {
				args = append(args, "--output", "json")
				mockConfigurationService.ConfigurationsReturns(nil)

				rootCfg := cmd.NewRootConfig()
				configurationCmd := cmd.NewConfigurationListCmd(mockConfigurationService, rootCfg)
				_, _, runErr := executeCmd(configurationCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
				Expect(rootCfg.Output.Value).To(Equal("json"))
			})
		})
	})
})
