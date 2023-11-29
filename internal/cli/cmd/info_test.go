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

	"github.com/epinio/epinio/internal/cli/cmd"
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/internal/cli/usercmd/usercmdfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var _ = Describe("Command 'epinio info'", func() {

	var (
		mock    *usercmdfakes.FakeAPIClient
		infoCmd *cobra.Command
		output  io.ReadWriter
	)

	BeforeEach(func() {
		epinioClient, err := usercmd.New()
		Expect(err).ToNot(HaveOccurred())

		mock = &usercmdfakes.FakeAPIClient{}
		epinioClient.API = mock

		output = &bytes.Buffer{}
		epinioClient.UI().SetOutput(output)

		infoCmd = cmd.NewInfoCmd(epinioClient, cmd.NewRootConfig())
	})

	When("the api returns a complete response", func() {
		It("will show all the info", func() {
			goodResponse := models.InfoResponse{
				Version:             "v1.2.3",
				KubeVersion:         "v1.22.33",
				Platform:            "k8s-platform",
				DefaultBuilderImage: "default-builder",
			}
			mock.InfoReturns(goodResponse, nil)

			stdout, _, _ := executeCmd(infoCmd, []string{}, output, nil)

			lines := strings.Split(stdout, "\n")
			Expect(lines).To(HaveLen(7), stdout)

			Expect(lines[0]).To(Equal("✔️  Epinio Environment"))
			Expect(lines[1]).To(Equal("Platform: k8s-platform"))
			Expect(lines[2]).To(Equal("Kubernetes Version: v1.22.33"))
			Expect(lines[3]).To(Equal("Epinio Server Version: v1.2.3"))
			Expect(lines[4]).To(Equal("Epinio Client Version: v0.0.0-dev"))
			Expect(lines[5]).To(Equal("OIDC enabled: false"))
		})
	})

	When("the api fails", func() {
		It("will show an error", func() {
			mock.InfoReturns(models.InfoResponse{}, errors.New("something failed"))

			_, outerr, _ := executeCmd(infoCmd, []string{}, nil, output)

			lines := strings.Split(outerr, "\n")
			Expect(lines).To(HaveLen(2), outerr)

			Expect(lines[0]).To(ContainSubstring("error retrieving Epinio environment information: something failed"))
		})
	})

})
