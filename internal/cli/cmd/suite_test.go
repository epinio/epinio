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
	"io"
	"testing"

	"github.com/spf13/cobra"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEpinio(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Epinio Suite CMD")
}

func executeCmd(cmd *cobra.Command, args []string, output, outputErr io.ReadWriter) (string, string, error) {
	GinkgoHelper()

	cmd.SetOut(output)
	cmd.SetErr(outputErr)
	cmd.SetArgs(args)

	// we don't check the err because if the command fails we want to check the error anyway
	runErr := cmd.Execute()

	var out, outErr []byte
	var err error

	if output != nil {
		out, err = io.ReadAll(output)
		Expect(err).ToNot(HaveOccurred())
	}

	if outputErr != nil {
		outErr, err = io.ReadAll(outputErr)
		Expect(err).ToNot(HaveOccurred())
	}

	return string(out), string(outErr), runErr
}
