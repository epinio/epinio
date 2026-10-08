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

package application

import (
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BoundConfigurationsWithType", func() {
	var configurationTypes map[ConfigurationKey]string

	BeforeEach(func() {
		configurationTypes = map[ConfigurationKey]string{
			EncodeConfigurationKey("mine", "workspace"):     "custom",
			EncodeConfigurationKey("db-creds", "workspace"): "service",
			EncodeConfigurationKey("mine", "other"):         "service",
		}
	})

	It("pairs each name with the type of its configuration", func() {
		Expect(BoundConfigurationsWithType(
			[]string{"mine", "db-creds"}, "workspace", configurationTypes,
		)).To(Equal([]models.BoundConfiguration{
			{Name: "mine", Type: "custom"},
			{Name: "db-creds", Type: "service"},
		}))
	})

	It("keys on the namespace, not just the name", func() {
		Expect(BoundConfigurationsWithType(
			[]string{"mine"}, "other", configurationTypes,
		)).To(Equal([]models.BoundConfiguration{
			{Name: "mine", Type: "service"},
		}))
	})

	It("keeps a name whose configuration is gone, with an empty type", func() {
		Expect(BoundConfigurationsWithType(
			[]string{"vanished"}, "workspace", configurationTypes,
		)).To(Equal([]models.BoundConfiguration{
			{Name: "vanished", Type: ""},
		}))
	})

	It("returns an empty slice for an app with no bound configurations", func() {
		Expect(BoundConfigurationsWithType(
			nil, "workspace", configurationTypes,
		)).To(BeEmpty())
	})
})
