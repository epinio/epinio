// Copyright © 2021 - 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/helpers/kubernetes/tailer"
)

// decodeStoredLines parses newline-delimited JSON as written by encodeStagingLogs.
func decodeStoredLines(data []byte) []tailer.ContainerLogLine {
	var lines []tailer.ContainerLogLine
	for _, raw := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if raw == "" {
			continue
		}
		var line tailer.ContainerLogLine
		Expect(json.Unmarshal([]byte(raw), &line)).To(Succeed())
		lines = append(lines, line)
	}
	return lines
}

var _ = Describe("Staging logs persistence", func() {
	Describe("NewStagingLogsSettings", func() {
		It("converts the size cap from MB to bytes", func() {
			settings := NewStagingLogsSettings(true, 2)
			Expect(settings.Enabled).To(BeTrue())
			Expect(settings.MaxBytes).To(Equal(int64(2 * 1024 * 1024)))
		})

		It("uses the default size when the configured size is not positive", func() {
			settings := NewStagingLogsSettings(true, 0)
			Expect(settings.MaxBytes).To(Equal(int64(defaultStagingLogsMaxSizeMB * 1024 * 1024)))
		})

		It("stays disabled when persisting is off", func() {
			Expect(NewStagingLogsSettings(false, 10).Enabled).To(BeFalse())
		})
	})

	Describe("object keys", func() {
		It("keys the logs of a stage by namespace and stage ID", func() {
			Expect(stagingLogObjectKey("workspace", "abc123")).To(Equal("staging-logs/workspace/abc123.ndjson"))
		})

		It("keys the pointer to the latest stage by namespace and app name", func() {
			Expect(stagingLogPointerKey("workspace", "sample")).To(Equal("staging-logs/workspace/apps/sample"))
		})
	})

	Describe("encodeStagingLogs", func() {
		It("stores every line when they fit into the size cap", func() {
			input := []tailer.ContainerLogLine{
				{Message: "first", ContainerName: "buildpack", PodName: "pod-a"},
				{Message: "second", ContainerName: "buildpack", PodName: "pod-a"},
			}

			data, err := encodeStagingLogs(input, 1024*1024)
			Expect(err).ToNot(HaveOccurred())

			stored := decodeStoredLines(data)
			Expect(stored).To(Equal(input))
		})

		It("keeps the most recent lines and prepends a notice when the cap is reached", func() {
			var input []tailer.ContainerLogLine
			for i := range 5 {
				input = append(input, tailer.ContainerLogLine{Message: fmt.Sprintf("message-%d", i)})
			}

			// The cap fits exactly the two most recent lines.
			lastTwo := 0
			for _, line := range input[3:] {
				data, err := json.Marshal(line)
				Expect(err).ToNot(HaveOccurred())
				lastTwo += len(data) + 1
			}

			data, err := encodeStagingLogs(input, int64(lastTwo))
			Expect(err).ToNot(HaveOccurred())

			stored := decodeStoredLines(data)
			Expect(stored).To(HaveLen(3))
			Expect(stored[0].Message).To(ContainSubstring("3 earlier log lines dropped"))
			Expect(stored[1].Message).To(Equal("message-3"))
			Expect(stored[2].Message).To(Equal("message-4"))
		})

		It("stores nothing but the notice when no line fits", func() {
			input := []tailer.ContainerLogLine{{Message: "a line that is too long for the cap"}}

			data, err := encodeStagingLogs(input, 1)
			Expect(err).ToNot(HaveOccurred())

			stored := decodeStoredLines(data)
			Expect(stored).To(HaveLen(1))
			Expect(stored[0].Message).To(ContainSubstring("1 earlier log lines dropped"))
		})

		It("stores an empty log without a notice", func() {
			data, err := encodeStagingLogs(nil, 1024)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(BeEmpty())
		})
	})
})
