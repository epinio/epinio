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

package s3manager_test

import (
	"errors"

	"github.com/epinio/epinio/internal/s3manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnectionDetails", func() {
	Describe("Validate", func() {
		var details s3manager.ConnectionDetails
		When("mandatory settings are partly set", func() {
			BeforeEach(func() {
				details = s3manager.ConnectionDetails{
					Endpoint:        "myendpoint",
					AccessKeyID:     "",
					SecretAccessKey: "",
					Bucket:          "",
				}
			})
			It("returns an error", func() {
				Expect(details.Validate()).To(MatchError("when specifying an external s3 server, you must set all mandatory S3 options"))
			})
		})
		When("mandatory settings are empty but there are optional set", func() {
			BeforeEach(func() {
				details = s3manager.ConnectionDetails{
					Endpoint:        "",
					AccessKeyID:     "",
					SecretAccessKey: "",
					Bucket:          "",
					Location:        "somelocation",
				}
			})
			It("returns an error", func() {
				Expect(details.Validate()).To(MatchError("do not specify options if using the internal S3 storage"))
			})
		})
		When("all settings are empty", func() {
			BeforeEach(func() {
				details = s3manager.ConnectionDetails{}
			})
			It("returns no error", func() {
				Expect(details.Validate()).ToNot(HaveOccurred())
			})
		})
		When("mandatory settings are full and some optional are set", func() {
			BeforeEach(func() {
				details = s3manager.ConnectionDetails{
					Endpoint:        "myendpoint",
					AccessKeyID:     "myaccesskey",
					SecretAccessKey: "myaccesssecret",
					Bucket:          "somebucket",
					Location:        "somelocation",
				}
			})
			It("returns no error", func() {
				Expect(details.Validate()).ToNot(HaveOccurred())
			})
		})
		When("mandatory settings are full and no optional are set", func() {
			BeforeEach(func() {
				details = s3manager.ConnectionDetails{
					Endpoint:        "myendpoint",
					AccessKeyID:     "myaccesskey",
					SecretAccessKey: "myaccesssecret",
					Bucket:          "mybucket",
					Location:        "",
				}
			})
			It("returns no error", func() {
				Expect(details.Validate()).ToNot(HaveOccurred())
			})
		})
	})
})

var _ = Describe("IsQuotaExceededError", func() {
	When("error is nil", func() {
		It("returns false", func() {
			Expect(s3manager.IsQuotaExceededError(nil)).To(BeFalse())
		})
	})

	When("error contains 'QuotaExceeded' (s3gw format)", func() {
		It("returns true", func() {
			err := errors.New("Error response code QuotaExceeded.")
			Expect(s3manager.IsQuotaExceededError(err)).To(BeTrue())
		})
	})

	When("error contains 'quota exceeded' (case insensitive)", func() {
		It("returns true for lowercase", func() {
			err := errors.New("quota exceeded on bucket")
			Expect(s3manager.IsQuotaExceededError(err)).To(BeTrue())
		})

		It("returns true for uppercase", func() {
			err := errors.New("QUOTA EXCEEDED")
			Expect(s3manager.IsQuotaExceededError(err)).To(BeTrue())
		})

		It("returns true for mixed case", func() {
			err := errors.New("Quota Exceeded")
			Expect(s3manager.IsQuotaExceededError(err)).To(BeTrue())
		})
	})

	When("error contains 'quota limit'", func() {
		It("returns true", func() {
			err := errors.New("quota limit reached for storage")
			Expect(s3manager.IsQuotaExceededError(err)).To(BeTrue())
		})
	})

	When("error contains 'insufficient storage'", func() {
		It("returns true", func() {
			err := errors.New("insufficient storage available")
			Expect(s3manager.IsQuotaExceededError(err)).To(BeTrue())
		})
	})

	When("error contains 'storage quota'", func() {
		It("returns true", func() {
			err := errors.New("storage quota has been exceeded")
			Expect(s3manager.IsQuotaExceededError(err)).To(BeTrue())
		})
	})

	When("error contains 'minimum free drive threshold' (Minio format)", func() {
		It("returns true", func() {
			err := errors.New("Storage backend has reached its minimum free drive threshold. Please delete a few objects to proceed.")
			Expect(s3manager.IsQuotaExceededError(err)).To(BeTrue())
		})
	})

	When("error is unrelated to quota", func() {
		It("returns false for connection errors", func() {
			err := errors.New("connection refused")
			Expect(s3manager.IsQuotaExceededError(err)).To(BeFalse())
		})

		It("returns false for access denied errors", func() {
			err := errors.New("access denied")
			Expect(s3manager.IsQuotaExceededError(err)).To(BeFalse())
		})

		It("returns false for not found errors", func() {
			err := errors.New("object not found")
			Expect(s3manager.IsQuotaExceededError(err)).To(BeFalse())
		})

		It("returns false for timeout errors", func() {
			err := errors.New("request timeout")
			Expect(s3manager.IsQuotaExceededError(err)).To(BeFalse())
		})
	})
})
