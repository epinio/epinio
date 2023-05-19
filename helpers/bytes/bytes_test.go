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

package bytes_test

import (
	"github.com/epinio/epinio/helpers/bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BytesIEC", func() {
	It("properly formats various ranges", func() {
		for _, testcase := range []struct {
			count  int64
			result string
		}{
			/*B*/ {1, "1 B"},
			/*K*/ {1 + 1024, "1.0 KiB"},
			/*M*/ {1 + 1024*1024, "1.0 MiB"},
			/*G*/ {1 + 1024*1024*1024, "1.0 GiB"},
			/*T*/ {1 + 1024*1024*1024*1024, "1.0 TiB"},
			/*P*/ {1 + 1024*1024*1024*1024*1024, "1.0 PiB"},
			/*E*/ {1 + 1024*1024*1024*1024*1024*1024, "1.0 EiB"},
		} {
			r := bytes.ByteCountIEC(testcase.count)
			Expect(r).To(Equal(testcase.result))
		}
	})
})
