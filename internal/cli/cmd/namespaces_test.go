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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Command 'epinio namespace'", func() {

	var (
		mockNamespaceService *cmdfakes.FakeNamespaceService
		output, outputErr    io.ReadWriter
		args                 []string
	)

	BeforeEach(func() {
		mockNamespaceService = &cmdfakes.FakeNamespaceService{}

		args = []string{}
	})

	Context("namespace create", func() {

		When("called with no args", func() {
			It("fails", func() {
				namespaceCmd := cmd.NewNamespaceCreateCmd(mockNamespaceService)
				_, _, runErr := executeCmd(namespaceCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("accepts 1 arg(s), received 0"))
			})
		})

		When("called with multiple args", func() {
			It("fails", func() {
				args = append(args, "something", "more")

				namespaceCmd := cmd.NewNamespaceCreateCmd(mockNamespaceService)
				_, _, runErr := executeCmd(namespaceCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("accepts 1 arg(s), received 2"))
			})
		})

		When("the namespace create fails", func() {
			It("returns an error", func() {
				args = append(args, "mynamespace")

				mockNamespaceService.CreateNamespaceStub = func(s string) error {
					Expect(s).To(Equal("mynamespace"))
					return errors.New("something bad happened")
				}

				namespaceCmd := cmd.NewNamespaceCreateCmd(mockNamespaceService)
				_, _, runErr := executeCmd(namespaceCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("error creating epinio-controlled namespace: something bad happened"))
			})
		})

		When("the namespace create succeed", func() {
			It("returns ok", func() {
				args = append(args, "mynamespace")

				mockNamespaceService.CreateNamespaceStub = func(s string) error {
					Expect(s).To(Equal("mynamespace"))
					return nil
				}

				namespaceCmd := cmd.NewNamespaceCreateCmd(mockNamespaceService)
				_, _, runErr := executeCmd(namespaceCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
			})
		})
	})

	Context("namespace list", func() {

		When("called with one or more args", func() {
			It("fails", func() {
				args = append(args, "one")

				namespaceCmd := cmd.NewNamespaceListCmd(mockNamespaceService, cmd.NewRootConfig())
				_, _, runErr := executeCmd(namespaceCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal(`unknown command "one" for "list"`))

				args = append(args, "two")

				_, _, runErr = executeCmd(namespaceCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal(`unknown command "one" for "list"`))
			})
		})

		When("the namespace list fails", func() {
			It("returns an error", func() {
				mockNamespaceService.NamespacesReturns(errors.New("something bad happened"))

				namespaceCmd := cmd.NewNamespaceListCmd(mockNamespaceService, cmd.NewRootConfig())
				_, _, runErr := executeCmd(namespaceCmd, args, output, outputErr)
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(Equal("error listing epinio-controlled namespaces: something bad happened"))
			})
		})

		When("the namespace returns ok", func() {
			It("doesn't returns an error", func() {
				mockNamespaceService.NamespacesReturns(nil)

				namespaceCmd := cmd.NewNamespaceListCmd(mockNamespaceService, cmd.NewRootConfig())
				_, _, runErr := executeCmd(namespaceCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
			})

			It("default output flag is 'text'", func() {
				mockNamespaceService.NamespacesReturns(nil)

				rootCfg := cmd.NewRootConfig()
				namespaceCmd := cmd.NewNamespaceListCmd(mockNamespaceService, rootCfg)
				_, _, runErr := executeCmd(namespaceCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
				Expect(rootCfg.Output.Value).To(Equal("text"))
			})
		})

		When("called with the output flag to json", func() {
			It("the output is 'json'", func() {
				args = append(args, "--output", "json")
				mockNamespaceService.NamespacesReturns(nil)

				rootCfg := cmd.NewRootConfig()
				namespaceCmd := cmd.NewNamespaceListCmd(mockNamespaceService, rootCfg)
				_, _, runErr := executeCmd(namespaceCmd, args, output, outputErr)
				Expect(runErr).ToNot(HaveOccurred())
				Expect(rootCfg.Output.Value).To(Equal("json"))
			})
		})
	})
})
