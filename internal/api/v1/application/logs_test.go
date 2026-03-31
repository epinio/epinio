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

package application_test

import (
	"math"
	"net/http"
	"time"

	"github.com/epinio/epinio/internal/api/v1/application"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Application Log API Endpoint unit tests", func() {
	var allowedOrigins []string
	var request *http.Request
	var theFunc func(r *http.Request) bool

	JustBeforeEach(func() {
		theFunc = application.CheckOriginFunc(allowedOrigins)
	})

	BeforeEach(func() {
		var err error
		request, err = http.NewRequest("GET", "https://somedomain.org", nil)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("CheckOriginFunc", func() {
		When("allowed origins is empty", func() {
			BeforeEach(func() {
				allowedOrigins = []string{}
				request.Header.Set("Origin", "https://somedomain.org")
			})
			It("returns true", func() {
				Expect(theFunc(request)).To(BeTrue())
			})
		})
		When("origin header is empty", func() {
			BeforeEach(func() {
				allowedOrigins = []string{"https://somedomain.org"}
				request.Header.Set("Origin", "")
			})
			It("returns true", func() {
				Expect(theFunc(request)).To(BeTrue())
			})
		})
		When("allowed origins include a '*'", func() {
			BeforeEach(func() {
				allowedOrigins = []string{"*", "https://somedomain.org"}
				request.Header.Set("Origin", "https://notthesamedomain.org")
			})
			It("returns true", func() {
				Expect(theFunc(request)).To(BeTrue())
			})
		})
		When("allowed origins match the header", func() {
			BeforeEach(func() {
				allowedOrigins = []string{"https://somedomain.org"}
				request.Header.Set("Origin", "https://somedomain.org")
			})
			It("returns true", func() {
				Expect(theFunc(request)).To(BeTrue())
			})
		})
		When("there is no match", func() {
			BeforeEach(func() {
				allowedOrigins = []string{"https://somedomain.org"}
				request.Header.Set("Origin", "https://notthesamedomain.org")
			})
			It("returns false", func() {
				Expect(theFunc(request)).To(BeFalse())
			})
		})
	})

	Describe("parseLogParameters", func() {
		Context("tail parameter", func() {
			It("parses valid positive tail parameter", func() {
				params, err := application.ParseLogParameters("10", "", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Tail).ToNot(BeNil())
				Expect(*params.Tail).To(Equal(int64(10)))
			})

			It("parses valid zero tail parameter", func() {
				params, err := application.ParseLogParameters("0", "", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Tail).ToNot(BeNil())
				Expect(*params.Tail).To(Equal(int64(0)))
			})

			It("rejects negative tail parameter", func() {
				_, err := application.ParseLogParameters("-10", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("must be non-negative"))
			})

			It("rejects invalid tail parameter", func() {
				_, err := application.ParseLogParameters("invalid", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid tail parameter"))
			})
		})

		Context("since parameter", func() {
			It("parses valid since duration", func() {
				params, err := application.ParseLogParameters("", "5m", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Since).ToNot(BeNil())
				Expect(*params.Since).To(Equal(5 * time.Minute))
			})

			It("parses zero duration", func() {
				params, err := application.ParseLogParameters("", "0s", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Since).ToNot(BeNil())
				Expect(*params.Since).To(Equal(time.Duration(0)))
			})

			It("rejects negative duration", func() {
				_, err := application.ParseLogParameters("", "-5m", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("must be non-negative"))
			})

			It("rejects invalid duration format", func() {
				_, err := application.ParseLogParameters("", "invalid", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since parameter"))
			})
		})

		Context("since_time parameter", func() {
			It("parses valid RFC3339 timestamp", func() {
				timeStr := "2024-01-15T10:00:00Z"
				params, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.SinceTime).ToNot(BeNil())
				expectedTime, _ := time.Parse(time.RFC3339, timeStr)
				Expect(*params.SinceTime).To(Equal(expectedTime))
			})

			It("accepts future timestamp (will be handled by returning no logs)", func() {
				// Test that future times are accepted at parse time
				futureTime := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
				params, err := application.ParseLogParameters("", "", futureTime)
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.SinceTime).ToNot(BeNil())
			})

			It("rejects invalid timestamp format", func() {
				_, err := application.ParseLogParameters("", "", "2024-01-15 10:00:00")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since_time parameter"))
				Expect(err.Error()).To(ContainSubstring("RFC3339"))
			})
		})

		Context("combined parameters", func() {
			It("parses multiple valid parameters", func() {
				params, err := application.ParseLogParameters("100", "10m", "2024-01-15T10:00:00Z")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Tail).ToNot(BeNil())
				Expect(*params.Tail).To(Equal(int64(100)))
				Expect(params.Since).ToNot(BeNil())
				Expect(*params.Since).To(Equal(10 * time.Minute))
				Expect(params.SinceTime).ToNot(BeNil())
			})

			It("returns empty params when all parameters are empty", func() {
				params, err := application.ParseLogParameters("", "", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Tail).To(BeNil())
				Expect(params.Since).To(BeNil())
				Expect(params.SinceTime).To(BeNil())
			})

			It("handles tail and since together", func() {
				params, err := application.ParseLogParameters("100", "1h", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Tail).ToNot(BeNil())
				Expect(*params.Tail).To(Equal(int64(100)))
				Expect(params.Since).ToNot(BeNil())
				Expect(*params.Since).To(Equal(1 * time.Hour))
			})

			It("handles tail and since_time together", func() {
				timeStr := "2024-01-15T10:00:00Z"
				params, err := application.ParseLogParameters("50", "", timeStr)
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Tail).ToNot(BeNil())
				Expect(*params.Tail).To(Equal(int64(50)))
				Expect(params.SinceTime).ToNot(BeNil())
			})
		})

		Context("edge cases", func() {
			It("rejects tail values exceeding maximum", func() {
				_, err := application.ParseLogParameters("99999999", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("exceeds maximum"))
				Expect(err.Error()).To(ContainSubstring("100000"))
			})

			It("accepts tail at maximum boundary (100000)", func() {
				params, err := application.ParseLogParameters("100000", "", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Tail).ToNot(BeNil())
				Expect(*params.Tail).To(Equal(int64(100000)))
			})

			It("rejects tail just above maximum (100000)", func() {
				_, err := application.ParseLogParameters("100001", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("exceeds maximum"))
			})

			It("accepts very large since durations", func() {
				// 10 years in hours
				params, err := application.ParseLogParameters("", "87600h", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Since).ToNot(BeNil())
				Expect(*params.Since).To(Equal(87600 * time.Hour))
			})

			It("handles since_time with timezone offset", func() {
				timeStr := "2024-01-15T10:00:00+05:30"
				params, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.SinceTime).ToNot(BeNil())
			})

			It("handles since_time in UTC explicitly", func() {
				timeStr := "2024-01-15T10:00:00Z"
				params, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.SinceTime).ToNot(BeNil())
				Expect(params.SinceTime.Location()).To(Equal(time.UTC))
			})

			It("rejects since_time without timezone", func() {
				timeStr := "2024-01-15T10:00:00"
				_, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since_time parameter"))
			})
		})

		Context("numeric parsing edge cases", func() {
			It("handles tail with leading zeros", func() {
				params, err := application.ParseLogParameters("0100", "", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Tail).ToNot(BeNil())
				Expect(*params.Tail).To(Equal(int64(100)))
			})

			It("handles tail with explicit plus sign", func() {
				params, err := application.ParseLogParameters("+50", "", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Tail).ToNot(BeNil())
				Expect(*params.Tail).To(Equal(int64(50)))
			})

			It("rejects tail with exponential notation", func() {
				_, err := application.ParseLogParameters("1e2", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid tail parameter"))
			})

			It("rejects tail with hexadecimal notation", func() {
				_, err := application.ParseLogParameters("0x64", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid tail parameter"))
			})

			It("rejects tail with floating point", func() {
				_, err := application.ParseLogParameters("100.5", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid tail parameter"))
			})

			It("handles tail at int64 max value", func() {
				// int64 max is 9223372036854775807, but our max is 100000
				// This tests that we handle very large int64 values correctly
				maxInt64Str := "9223372036854775807"
				_, err := application.ParseLogParameters(maxInt64Str, "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("exceeds maximum"))
			})

			It("rejects tail beyond int64 max", func() {
				// This will fail at parsing stage
				_, err := application.ParseLogParameters("9223372036854775808", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid tail parameter"))
			})
		})

		Context("duration precision edge cases", func() {
			It("handles fractional hours", func() {
				params, err := application.ParseLogParameters("", "1.5h", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Since).ToNot(BeNil())
				Expect(*params.Since).To(Equal(90 * time.Minute))
			})

			It("handles nanoseconds", func() {
				params, err := application.ParseLogParameters("", "1000ns", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Since).ToNot(BeNil())
				Expect(*params.Since).To(Equal(1000 * time.Nanosecond))
			})

			It("handles milliseconds", func() {
				params, err := application.ParseLogParameters("", "500ms", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Since).ToNot(BeNil())
				Expect(*params.Since).To(Equal(500 * time.Millisecond))
			})

			It("handles mixed duration units", func() {
				params, err := application.ParseLogParameters("", "1h30m45s", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Since).ToNot(BeNil())
				expectedDuration := 1*time.Hour + 30*time.Minute + 45*time.Second
				Expect(*params.Since).To(Equal(expectedDuration))
			})

			It("rejects invalid duration units", func() {
				_, err := application.ParseLogParameters("", "5x", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since parameter"))
			})

			It("rejects duration with only number (no unit)", func() {
				_, err := application.ParseLogParameters("", "300", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since parameter"))
			})
		})

		Context("timestamp boundary edge cases", func() {
			It("handles Unix epoch time", func() {
				timeStr := "1970-01-01T00:00:00Z"
				params, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.SinceTime).ToNot(BeNil())
				Expect(params.SinceTime.Unix()).To(Equal(int64(0)))
			})

			It("handles very far future date", func() {
				timeStr := "2999-12-31T23:59:59Z"
				params, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.SinceTime).ToNot(BeNil())
			})

			It("handles extreme negative timezone offset", func() {
				timeStr := "2024-11-19T10:00:00-12:00"
				params, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.SinceTime).ToNot(BeNil())
			})

			It("handles extreme positive timezone offset", func() {
				timeStr := "2024-11-19T10:00:00+14:00"
				params, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.SinceTime).ToNot(BeNil())
			})

			It("handles fractional seconds", func() {
				timeStr := "2024-11-19T10:00:00.123456Z"
				params, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.SinceTime).ToNot(BeNil())
			})

			It("rejects invalid date (Feb 30)", func() {
				timeStr := "2024-02-30T10:00:00Z"
				_, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since_time parameter"))
			})

			It("rejects invalid time (25:00:00)", func() {
				timeStr := "2024-11-19T25:00:00Z"
				_, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since_time parameter"))
			})
		})

		Context("special character and encoding edge cases", func() {
			It("rejects tail with whitespace", func() {
				_, err := application.ParseLogParameters("10 0", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid tail parameter"))
			})

			It("rejects tail with comma separator", func() {
				_, err := application.ParseLogParameters("1,000", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid tail parameter"))
			})

			It("rejects tail with special characters", func() {
				_, err := application.ParseLogParameters("100!", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid tail parameter"))
			})

			It("handles since with various valid separators", func() {
				// Go's time.ParseDuration accepts no spaces
				_, err := application.ParseLogParameters("", "1 h", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since parameter"))
			})
		})

		Context("boundary value combinations", func() {
			It("handles maximum tail with minimum since", func() {
				params, err := application.ParseLogParameters("100000", "1ns", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(*params.Tail).To(Equal(int64(100000)))
				Expect(*params.Since).To(Equal(1 * time.Nanosecond))
			})

			It("handles tail=1 with very large since", func() {
				params, err := application.ParseLogParameters("1", "876000h", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(*params.Tail).To(Equal(int64(1)))
				Expect(*params.Since).To(Equal(876000 * time.Hour)) // 100 years
			})

			It("handles tail=0 with since=0s (return nothing)", func() {
				params, err := application.ParseLogParameters("0", "0s", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(*params.Tail).To(Equal(int64(0)))
				Expect(*params.Since).To(Equal(time.Duration(0)))
			})
		})

		Context("parameter parsing robustness", func() {
			It("handles empty string vs nil correctly", func() {
				// Empty strings should be treated as "not provided"
				params, err := application.ParseLogParameters("", "", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.Tail).To(BeNil())
				Expect(params.Since).To(BeNil())
				Expect(params.SinceTime).To(BeNil())
			})

			It("rejects malformed since_time with garbage", func() {
				_, err := application.ParseLogParameters("", "", "garbage")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since_time parameter"))
			})

			It("rejects since_time with only date", func() {
				_, err := application.ParseLogParameters("", "", "2024-11-19")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since_time parameter"))
			})

			It("rejects since_time with only time", func() {
				_, err := application.ParseLogParameters("", "", "10:00:00Z")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since_time parameter"))
			})

			It("handles leap year date correctly", func() {
				// Feb 29 exists only in leap years
				timeStr := "2024-02-29T10:00:00Z" // 2024 is a leap year
				params, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.SinceTime).ToNot(BeNil())
			})

			It("rejects Feb 29 in non-leap year", func() {
				timeStr := "2023-02-29T10:00:00Z" // 2023 is not a leap year
				_, err := application.ParseLogParameters("", "", timeStr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since_time parameter"))
			})
		})

		Context("mathematical edge cases", func() {
			It("handles tail at boundary of 100000", func() {
				// Test exact boundary
				params, err := application.ParseLogParameters("100000", "", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(*params.Tail).To(Equal(int64(100000)))
			})

			It("rejects tail at 100 001 (just over limit)", func() {
				_, err := application.ParseLogParameters("100001", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("exceeds maximum"))
			})

			It("handles very specific nanosecond precision", func() {
				// Test that precision is maintained
				params, err := application.ParseLogParameters("", "1ns", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(*params.Since).To(Equal(time.Nanosecond))
			})

			It("handles maximum reasonable duration", func() {
				// Duration that is valid but very large
				// Max duration is about 290 years
				maxHours := int64(math.MaxInt64 / int64(time.Hour))
				if maxHours > 876000 { // Cap at 100 years for test
					maxHours = 876000
				}
				params, err := application.ParseLogParameters("", "876000h", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
			})
		})

		Context("real-world scenario combinations", func() {
			It("handles typical usage: last 100 lines from past hour", func() {
				params, err := application.ParseLogParameters("100", "1h", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(*params.Tail).To(Equal(int64(100)))
				Expect(*params.Since).To(Equal(1 * time.Hour))
			})

			It("handles debugging scenario: last 1000 lines from past 5 minutes", func() {
				params, err := application.ParseLogParameters("1000", "5m", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(*params.Tail).To(Equal(int64(1000)))
				Expect(*params.Since).To(Equal(5 * time.Minute))
			})

			It("handles incident investigation: specific timestamp with large tail", func() {
				timeStr := "2024-11-19T14:30:00Z"
				params, err := application.ParseLogParameters("5000", "", timeStr)
				Expect(err).ToNot(HaveOccurred())
				Expect(*params.Tail).To(Equal(int64(5000)))
				Expect(params.SinceTime).ToNot(BeNil())
			})

			It("handles recent logs: past 30 seconds", func() {
				params, err := application.ParseLogParameters("", "30s", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(*params.Since).To(Equal(30 * time.Second))
			})

			It("handles date range query with both params", func() {
				// User wants last 500 lines since a specific time
				timeStr := "2024-11-19T10:00:00Z"
				params, err := application.ParseLogParameters("500", "2h", timeStr)
				Expect(err).ToNot(HaveOccurred())
				Expect(*params.Tail).To(Equal(int64(500)))
				Expect(*params.Since).ToNot(BeNil())    // Also set but will be ignored
				Expect(params.SinceTime).ToNot(BeNil()) // Takes precedence
			})
		})

		Context("error message quality", func() {
			It("provides clear error for negative tail", func() {
				_, err := application.ParseLogParameters("-5", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("must be non-negative"))
				Expect(err.Error()).To(ContainSubstring("-5"))
			})

			It("provides clear error for excessive tail", func() {
				_, err := application.ParseLogParameters("100001", "", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("exceeds maximum"))
				Expect(err.Error()).To(ContainSubstring("100000"))
				Expect(err.Error()).To(ContainSubstring("100001"))
			})

			It("provides clear error for invalid since format", func() {
				_, err := application.ParseLogParameters("", "invalid", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since parameter"))
				Expect(err.Error()).To(ContainSubstring("invalid"))
			})

			It("provides clear error for malformed timestamp", func() {
				_, err := application.ParseLogParameters("", "", "not-a-date")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid since_time parameter"))
				Expect(err.Error()).To(ContainSubstring("RFC3339"))
			})
		})

		Context("container filtering parameters", func() {
			It("parses valid include_containers parameter", func() {
				params, err := application.ParseLogParametersForTest("", "", "", "app-container,worker-container", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.IncludeContainers).To(HaveLen(2))
				Expect(params.IncludeContainers).To(ContainElements("app-container", "worker-container"))
			})

			It("parses valid exclude_containers parameter", func() {
				params, err := application.ParseLogParametersForTest("", "", "", "", "istio-proxy,linkerd-proxy")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.ExcludeContainers).To(HaveLen(2))
				Expect(params.ExcludeContainers).To(ContainElements("istio-proxy", "linkerd-proxy"))
			})

			It("parses both include and exclude containers", func() {
				params, err := application.ParseLogParametersForTest("", "", "", "app-container", "istio-proxy")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.IncludeContainers).To(HaveLen(1))
				Expect(params.IncludeContainers).To(ContainElement("app-container"))
				Expect(params.ExcludeContainers).To(HaveLen(1))
				Expect(params.ExcludeContainers).To(ContainElement("istio-proxy"))
			})

			It("filters out empty strings in include_containers", func() {
				params, err := application.ParseLogParametersForTest("", "", "", "app-container,,worker-container,", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.IncludeContainers).To(HaveLen(2))
				Expect(params.IncludeContainers).To(ContainElements("app-container", "worker-container"))
			})

			It("filters out empty strings in exclude_containers", func() {
				params, err := application.ParseLogParametersForTest("", "", "", "", "istio-proxy,,linkerd-proxy,")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.ExcludeContainers).To(HaveLen(2))
				Expect(params.ExcludeContainers).To(ContainElements("istio-proxy", "linkerd-proxy"))
			})

			It("trims whitespace from container names", func() {
				params, err := application.ParseLogParametersForTest("", "", "", " app-container , worker-container ", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.IncludeContainers).To(HaveLen(2))
				Expect(params.IncludeContainers).To(ContainElements("app-container", "worker-container"))
			})

			It("handles single container name", func() {
				params, err := application.ParseLogParametersForTest("", "", "", "app-container", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.IncludeContainers).To(HaveLen(1))
				Expect(params.IncludeContainers).To(ContainElement("app-container"))
			})

			It("handles all empty strings in include_containers", func() {
				params, err := application.ParseLogParametersForTest("", "", "", ",,", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.IncludeContainers).To(BeEmpty())
			})

			It("handles all empty strings in exclude_containers", func() {
				params, err := application.ParseLogParametersForTest("", "", "", "", ",,")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.ExcludeContainers).To(BeEmpty())
			})

			It("handles regex patterns in include_containers", func() {
				params, err := application.ParseLogParametersForTest("", "", "", "app-.*,worker-.*", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.IncludeContainers).To(HaveLen(2))
				Expect(params.IncludeContainers).To(ContainElements("app-.*", "worker-.*"))
			})

			It("handles regex patterns in exclude_containers", func() {
				params, err := application.ParseLogParametersForTest("", "", "", "", "istio-.*,linkerd-.*")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.ExcludeContainers).To(HaveLen(2))
				Expect(params.ExcludeContainers).To(ContainElements("istio-.*", "linkerd-.*"))
			})

			It("handles container names with special characters", func() {
				params, err := application.ParseLogParametersForTest("", "", "", "app-container-v1.2.3,worker_container", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(params.IncludeContainers).To(HaveLen(2))
				Expect(params.IncludeContainers).To(ContainElements("app-container-v1.2.3", "worker_container"))
			})

			It("combines container filters with other parameters", func() {
				params, err := application.ParseLogParametersForTest("100", "1h", "", "app-container", "istio-proxy")
				Expect(err).ToNot(HaveOccurred())
				Expect(params).ToNot(BeNil())
				Expect(*params.Tail).To(Equal(int64(100)))
				Expect(*params.Since).To(Equal(1 * time.Hour))
				Expect(params.IncludeContainers).To(HaveLen(1))
				Expect(params.ExcludeContainers).To(HaveLen(1))
			})
		})
	})
})
