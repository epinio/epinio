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

package usercmd_test

import (
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/internal/cli/usercmd/usercmdfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client Configurations unit tests", func() {
	var (
		fake         *usercmdfakes.FakeAPIClient
		epinioClient *usercmd.EpinioClient
	)

	BeforeEach(func() {
		fake = &usercmdfakes.FakeAPIClient{}
		fake.ConfigurationCreateReturns(models.Response{}, nil)
		fake.ConfigurationUpdateReturns(models.Response{}, nil)

		var err error
		epinioClient, err = usercmd.New()
		Expect(err).ToNot(HaveOccurred())

		epinioClient.Settings = &settings.Settings{
			Namespace: "workspace",
			API:       "https://epinio.example.com",
			User:      "epinio",
		}
		epinioClient.API = fake
	})

	Describe("CreateConfiguration", func() {

		It("keeps values containing '=' intact", func() {
			err := epinioClient.CreateConfiguration("training-auth", []string{
				"BETTER_AUTH_SECRET", "HTBCcNaoiW+piphiHQZSVq+JelIp3F6W4/FV1rfWdQI=",
				"GOOGLE_CLIENT_SECRET", "GOCSPX-secret",
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(fake.ConfigurationCreateCallCount()).To(Equal(1))
			request, namespace := fake.ConfigurationCreateArgsForCall(0)
			Expect(namespace).To(Equal("workspace"))
			Expect(request.Name).To(Equal("training-auth"))
			Expect(request.Data).To(Equal(map[string]string{
				"BETTER_AUTH_SECRET":   "HTBCcNaoiW+piphiHQZSVq+JelIp3F6W4/FV1rfWdQI=",
				"GOOGLE_CLIENT_SECRET": "GOCSPX-secret",
			}))
		})

		It("rejects a key kubernetes would not accept, without calling the API", func() {
			err := epinioClient.CreateConfiguration("training-auth", []string{
				"BETTER_AUTH_SECRET=HTBCcNaoiW+piphiHQZSVq+JelIp3F6W4/FV1rfWdQI=", "value",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(
				"invalid configuration key `BETTER_AUTH_SECRET=HTBCcNaoiW+piphiHQZSVq+JelIp3F6W4/FV1rfWdQI=`"))

			Expect(fake.ConfigurationCreateCallCount()).To(Equal(0))
		})

		It("rejects a key/value dictionary with a dangling key", func() {
			err := epinioClient.CreateConfiguration("training-auth", []string{"lonely"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("bad key/value dictionary, last key has no value"))

			Expect(fake.ConfigurationCreateCallCount()).To(Equal(0))
		})
	})

	Describe("UpdateConfiguration", func() {

		It("keeps values containing '=' intact", func() {
			err := epinioClient.UpdateConfiguration("training-auth", nil, map[string]string{
				"BETTER_AUTH_SECRET": "HTBCcNaoiW+piphiHQZSVq+JelIp3F6W4/FV1rfWdQI=",
			}, false)
			Expect(err).ToNot(HaveOccurred())

			Expect(fake.ConfigurationUpdateCallCount()).To(Equal(1))
			request, namespace, name := fake.ConfigurationUpdateArgsForCall(0)
			Expect(namespace).To(Equal("workspace"))
			Expect(name).To(Equal("training-auth"))
			Expect(request.Set).To(Equal(map[string]string{
				"BETTER_AUTH_SECRET": "HTBCcNaoiW+piphiHQZSVq+JelIp3F6W4/FV1rfWdQI=",
			}))
		})

		It("rejects a key kubernetes would not accept, without calling the API", func() {
			err := epinioClient.UpdateConfiguration("training-auth", nil, map[string]string{
				"BETTER AUTH SECRET": "value",
			}, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid configuration key `BETTER AUTH SECRET`"))

			Expect(fake.ConfigurationUpdateCallCount()).To(Equal(0))
		})
	})
})
