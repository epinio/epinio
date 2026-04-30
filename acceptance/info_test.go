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

package acceptance_test

import (
	"encoding/json"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Info", LMisc, func() {
	It("succeeds", func() {
		out, err := env.Epinio("", "info")
		Expect(err).ToNot(HaveOccurred(), out)
		Expect(out).To(ContainSubstring(`Epinio Environment`))
		Expect(out).To(ContainSubstring(`Platform: `))
		Expect(out).To(ContainSubstring(`Kubernetes Version: `))
		Expect(out).To(ContainSubstring(`Epinio Server Version: `))
		Expect(out).To(ContainSubstring(`Epinio Client Version: `))
		Expect(out).To(ContainSubstring(`OIDC enabled: `))
	})

	It("succeeds with JSON", func() {
		out, err := env.Epinio("", "info", "--output", "json")
		Expect(err).ToNot(HaveOccurred(), out)

		info := &models.InfoResponse{}
		err = json.Unmarshal([]byte(out), info)
		Expect(err).ToNot(HaveOccurred(), out)
		Expect(info.Platform).ToNot(BeEmpty())
		Expect(info.Version).ToNot(BeEmpty())
	})
})
