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

var _ = Describe("Command 'epinio app'", func() {

	var (
		mockAppService    *cmdfakes.FakeApplicationsService
		mockAPIClient     *usercmdfakes.FakeAPIClient
		output, outputErr io.ReadWriter
		args              []string
	)

	BeforeEach(func() {
		mockAppService = &cmdfakes.FakeApplicationsService{}
		mockAPIClient = &usercmdfakes.FakeAPIClient{}
		mockAppService.GetAPIReturns(mockAPIClient)

		args = []string{}
	})

	// TODO: bind, unbind, update, delete, port-forward

	Context("app create", func() {

		When("called with no args", func() {
			It("fails", func() {
				appCmd := cmd.NewAppCreateCmd(mockAppService)
				_, _, runErr := executeCmd(appCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("accepts 1 arg(s), received 0"))
			})
		})

		When("called with more than 2 args", func() {
			It("fails", func() {
				args = append(args, "something", "more")

				appCmd := cmd.NewAppCreateCmd(mockAppService)
				_, _, runErr := executeCmd(appCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("accepts 1 arg(s), received 2"))
			})
		})

		When("the app create fails", func() {
			It("returns an error", func() {
				args = append(args, "myapp")

				mockAppService.AppCreateStub = func(name string, updateRequest models.ApplicationUpdateRequest) error {
					Expect(name).To(Equal("myapp"))
					return errors.New("something bad happened")
				}

				appCmd := cmd.NewAppCreateCmd(mockAppService)
				_, _, runErr := executeCmd(appCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("error creating app: something bad happened"))
			})
		})

		When("the app create succeeds", func() {
			It("returns ok", func() {
				args = append(args, "myapp")

				mockAppService.AppCreateStub = func(name string, updateRequest models.ApplicationUpdateRequest) error {
					Expect(name).To(Equal("myapp"))
					return nil
				}

				appCmd := cmd.NewAppCreateCmd(mockAppService)
				_, _, runErr := executeCmd(appCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
			})
		})
	})

	Context("app list", func() {

		When("called with one or more args", func() {
			It("fails", func() {
				args = append(args, "one")

				appCmd := cmd.NewAppListCmd(mockAppService, cmd.NewRootConfig())
				_, _, runErr := executeCmd(appCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal(`accepts 0 arg(s), received 1`))

				args = append(args, "two")

				_, _, runErr = executeCmd(appCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal(`accepts 0 arg(s), received 2`))
			})
		})

		When("the app list fails", func() {
			It("returns an error", func() {
				mockAppService.AppsReturns(errors.New("something bad happened"))

				appCmd := cmd.NewAppListCmd(mockAppService, cmd.NewRootConfig())
				_, _, runErr := executeCmd(appCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("error listing apps: something bad happened"))
			})
		})

		When("the app returns ok", func() {
			It("does not return an error", func() {
				mockAppService.AppsReturns(nil)

				appCmd := cmd.NewAppListCmd(mockAppService, cmd.NewRootConfig())
				_, _, runErr := executeCmd(appCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
			})

			It("shows that the default output flag is 'text'", func() {
				mockAppService.AppsReturns(nil)

				rootCfg := cmd.NewRootConfig()
				appCmd := cmd.NewAppListCmd(mockAppService, rootCfg)
				_, _, runErr := executeCmd(appCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
				Expect(rootCfg.Output.Value).To(Equal("text"))
			})
		})

		When("called with the output flag to json", func() {
			It("shows that the output is 'json'", func() {
				args = append(args, "--output", "json")
				mockAppService.AppsReturns(nil)

				rootCfg := cmd.NewRootConfig()
				appCmd := cmd.NewAppListCmd(mockAppService, rootCfg)
				_, _, runErr := executeCmd(appCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
				Expect(rootCfg.Output.Value).To(Equal("json"))
			})
		})
	})
})
