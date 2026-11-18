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

package client_test

import (
	"context"
	"net/http"

	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client settings headers", func() {
	It("applies headers persisted in settings on New", func() {
		epinioClient := client.New(context.Background(), &settings.Settings{
			API:      "https://epinio.example.com",
			Location: "fake",
			Headers: http.Header{
				"Proxy-Authorization": []string{"Bearer settings-token"},
			},
		})

		Expect(epinioClient.Headers().Get("Proxy-Authorization")).To(Equal("Bearer settings-token"))
	})

	It("keeps an empty header map when settings have no headers", func() {
		epinioClient := client.New(context.Background(), &settings.Settings{
			API:      "https://epinio.example.com",
			Location: "fake",
		})

		Expect(epinioClient.Headers()).To(Equal(http.Header{}))
	})
})
