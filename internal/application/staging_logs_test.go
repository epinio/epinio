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
	"time"

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

			data, err := encodeStagingLogs(input, 1024*1024, 0)
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

			data, err := encodeStagingLogs(input, int64(lastTwo), 0)
			Expect(err).ToNot(HaveOccurred())

			stored := decodeStoredLines(data)
			Expect(stored).To(HaveLen(3))
			Expect(stored[0].Message).To(ContainSubstring("3 earlier log lines dropped"))
			Expect(stored[1].Message).To(Equal("message-3"))
			Expect(stored[2].Message).To(Equal("message-4"))
		})

		It("stores nothing but the notice when no line fits", func() {
			input := []tailer.ContainerLogLine{{Message: "a line that is too long for the cap"}}

			data, err := encodeStagingLogs(input, 1, 0)
			Expect(err).ToNot(HaveOccurred())

			stored := decodeStoredLines(data)
			Expect(stored).To(HaveLen(1))
			Expect(stored[0].Message).To(ContainSubstring("1 earlier log lines dropped"))
		})

		It("stores an empty log without a notice", func() {
			data, err := encodeStagingLogs(nil, 1024, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(BeEmpty())
		})

		It("preserves the read-time drop count in the notice when remaining lines fit", func() {
			kept := []tailer.ContainerLogLine{
				{Message: "kept-1"},
				{Message: "kept-2"},
			}

			data, err := encodeStagingLogs(kept, 1024*1024, 7)
			Expect(err).ToNot(HaveOccurred())

			stored := decodeStoredLines(data)
			Expect(stored).To(HaveLen(3))
			Expect(stored[0].Message).To(ContainSubstring("7 earlier log lines dropped"))
			Expect(stored[1].Message).To(Equal("kept-1"))
			Expect(stored[2].Message).To(Equal("kept-2"))
		})
	})

	Describe("stagingLogBuffer", func() {
		It("keeps only the newest lines that fit the max bytes while reading", func() {
			var input []tailer.ContainerLogLine
			for i := range 5 {
				input = append(input, tailer.ContainerLogLine{Message: fmt.Sprintf("message-%d", i)})
			}

			lastTwo := int64(0)
			for _, line := range input[3:] {
				data, err := json.Marshal(line)
				Expect(err).ToNot(HaveOccurred())
				lastTwo += int64(len(data) + 1)
			}

			buf := stagingLogBuffer{max: lastTwo}
			for _, line := range input {
				Expect(buf.add(line)).To(Succeed())
			}

			Expect(buf.lines).To(HaveLen(2))
			Expect(buf.dropped).To(Equal(3))
			Expect(buf.lines[0].Message).To(Equal("message-3"))
			Expect(buf.lines[1].Message).To(Equal("message-4"))

			// Collect already trimmed: encode must still emit the drop notice.
			data, err := encodeStagingLogs(buf.lines, lastTwo, buf.dropped)
			Expect(err).ToNot(HaveOccurred())
			stored := decodeStoredLines(data)
			Expect(stored[0].Message).To(ContainSubstring("3 earlier log lines dropped"))
			Expect(stored).To(HaveLen(3))
		})

		It("drops a sole remaining line that alone exceeds the cap", func() {
			buf := stagingLogBuffer{max: 1}
			Expect(buf.add(tailer.ContainerLogLine{Message: "too-large"})).To(Succeed())
			Expect(buf.lines).To(BeEmpty())
			Expect(buf.dropped).To(Equal(1))

			data, err := encodeStagingLogs(buf.lines, 1, buf.dropped)
			Expect(err).ToNot(HaveOccurred())
			stored := decodeStoredLines(data)
			Expect(stored).To(HaveLen(1))
			Expect(stored[0].Message).To(ContainSubstring("1 earlier log lines dropped"))
		})
	})

	Describe("FilterStoredStagingLogs", func() {
		base := []tailer.ContainerLogLine{
			{Message: "old", ContainerName: "buildpack", Timestamp: time.Now().Add(-2 * time.Hour).Format(time.RFC3339Nano)},
			{Message: "new-build", ContainerName: "buildpack", Timestamp: time.Now().Format(time.RFC3339Nano)},
			{Message: "new-download", ContainerName: "download-s3-blob", Timestamp: time.Now().Format(time.RFC3339Nano)},
		}

		It("filters by include_containers", func() {
			filtered, err := FilterStoredStagingLogs(base, &LogParameters{
				IncludeContainers: []string{"download-s3-blob"},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(filtered).To(HaveLen(1))
			Expect(filtered[0].Message).To(Equal("new-download"))
		})

		It("filters by since and tail", func() {
			since := time.Hour
			tail := int64(1)
			filtered, err := FilterStoredStagingLogs(base, &LogParameters{
				Since: &since,
				Tail:  &tail,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(filtered).To(HaveLen(1))
			Expect(filtered[0].Message).To(Equal("new-download"))
		})

		It("treats tail 0 as no limit", func() {
			tail := int64(0)
			filtered, err := FilterStoredStagingLogs(base, &LogParameters{Tail: &tail})
			Expect(err).ToNot(HaveOccurred())
			Expect(filtered).To(HaveLen(len(base)))
		})
	})

	Describe("encode decode round trip", func() {
		It("round-trips encoded staging log lines", func() {
			input := []tailer.ContainerLogLine{
				{Message: "one", ContainerName: "buildpack"},
				{Message: "two", ContainerName: "buildpack"},
			}

			data, err := encodeStagingLogs(input, 1024*1024, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).ToNot(BeEmpty())
			Expect(decodeStoredLines(data)).To(Equal(input))
		})
	})
})
