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

// Package auth collects structures and functions around the
// generation and processing of credentials.
package auth_test

import (
	"github.com/epinio/epinio/internal/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auth actions", func() {
	Describe("InitActions", func() {
		It("works", func() {
			actions, err := auth.InitActions()
			Expect(err).ToNot(HaveOccurred())
			Expect(actions).ToNot(BeEmpty())
		})

		It("keeps app_write as compatibility action", func() {
			_, err := auth.InitActions()
			Expect(err).ToNot(HaveOccurred())

			appWrite, found := auth.ActionsMap["app_write"]
			Expect(found).To(BeTrue())

			// app_write is now a composite action and should still cover existing write paths.
			Expect(appWrite.Routes).To(ContainElement("AppCreate"))
			Expect(appWrite.Routes).To(ContainElement("AppDelete"))
			Expect(appWrite.Routes).To(ContainElement("AppDeploy"))
			Expect(appWrite.Routes).To(ContainElement("AppStage"))
			Expect(appWrite.Routes).To(ContainElement("AppUpdate"))
			Expect(appWrite.Routes).To(ContainElement("EnvSet"))
			Expect(appWrite.Routes).To(ContainElement("ConfigurationBindingCreate"))
			// Keep legacy dependency behavior too.
			Expect(appWrite.Routes).To(ContainElement("AppShow"))
			Expect(appWrite.WsRoutes).To(ContainElement("AppLogs"))
		})
	})
})
