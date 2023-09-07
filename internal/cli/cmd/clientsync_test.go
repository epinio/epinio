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
	"bytes"
	"io"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/internal/cli/cmd"
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/internal/cli/usercmd/usercmdfakes"
	"github.com/epinio/epinio/internal/selfupdater/selfupdaterfakes"
	"github.com/epinio/epinio/internal/version"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type FakeUpdater struct {
	err error
}

func (f *FakeUpdater) Update(string) error {
	return f.err
}

var _ = Describe("Command 'epinio client-sync'", func() {

	var (
		mock          *usercmdfakes.FakeAPIClient
		mockUpdater   *selfupdaterfakes.FakeUpdater
		clientSyncCmd *cobra.Command
		output        io.ReadWriter
	)

	BeforeEach(func() {
		epinioClient, err := usercmd.New()
		Expect(err).ToNot(HaveOccurred())

		mock = &usercmdfakes.FakeAPIClient{}
		epinioClient.API = mock

		output = &bytes.Buffer{}
		epinioClient.UI().SetOutput(output)

		mockUpdater = &selfupdaterfakes.FakeUpdater{}
		epinioClient.Updater = mockUpdater

		clientSyncCmd = cmd.NewClientSyncCmd(epinioClient)
	})

	When("the api returns an info response", func() {

		var serverVersion string

		BeforeEach(func() {
			serverVersion = "v1.2.3"

			goodResponse := models.InfoResponse{
				Version:             serverVersion,
				KubeVersion:         "v1.22.33",
				Platform:            "k8s-platform",
				DefaultBuilderImage: "default-builder",
			}
			mock.InfoReturns(goodResponse, nil)
		})

		Describe("and the versions are different", func() {

			BeforeEach(func() {
				initialVersion := version.Version
				version.Version = "v0.0.0"

				DeferCleanup(func() {
					version.Version = initialVersion
				})
			})

			Describe("it calls the updater", func() {
				When("the updater succeed", func() {
					It("shows the final version", func() {
						mockUpdater.UpdateReturns(nil)

						args := []string{"client-sync"}
						stdout, _, _ := executeCmd(clientSyncCmd, args, output, nil)

						Expect(mockUpdater.UpdateCallCount()).To(Equal(1))

						lines := strings.Split(stdout, "\n")
						Expect(lines).To(HaveLen(2))

						Expect(lines[0]).To(Equal("✔️  Updated epinio client to " + serverVersion))
					})
				})

				When("the updater fails", func() {
					It("shows an error", func() {
						mockUpdater.UpdateReturns(errors.New("updater failed"))

						args := []string{"client-sync"}
						_, stderr, _ := executeCmd(clientSyncCmd, args, nil, output)

						Expect(mockUpdater.UpdateCallCount()).To(Equal(1))

						lines := strings.Split(stderr, "\n")
						Expect(lines).To(HaveLen(2))

						Expect(lines[0]).To(ContainSubstring("error syncing the Epinio client: updating the client: updater failed"))
					})
				})
			})
		})

		Describe("and the versions are the same", func() {

			BeforeEach(func() {
				initialVersion := version.Version
				version.Version = serverVersion

				DeferCleanup(func() {
					version.Version = initialVersion
				})
			})

			Describe("it doesn't call the updater", func() {
				It("shows the final version", func() {
					mockUpdater.UpdateReturns(nil)

					args := []string{"client-sync"}
					stdout, _, _ := executeCmd(clientSyncCmd, args, output, nil)

					Expect(mockUpdater.UpdateCallCount()).To(Equal(0))

					lines := strings.Split(stdout, "\n")
					Expect(lines).To(HaveLen(2))

					Expect(lines[0]).To(Equal("✔️  Client and server version are the same (v1.2.3). Nothing to do!"))
				})
			})
		})
	})

	When("the api fails", func() {
		It("will show an error", func() {
			mock.InfoReturns(models.InfoResponse{}, errors.New("something failed"))

			args := []string{"client-sync"}
			_, stderr, _ := executeCmd(clientSyncCmd, args, nil, output)

			lines := strings.Split(stderr, "\n")
			Expect(lines).To(HaveLen(2))

			Expect(lines[0]).To(ContainSubstring("error syncing the Epinio client: something failed"))
		})
	})
})
