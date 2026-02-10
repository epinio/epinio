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

		It("defines granular application actions with the expected routes", func() {
			_, err := auth.InitActions()
			Expect(err).ToNot(HaveOccurred())

			appCreate, found := auth.ActionsMap["app_create"]
			Expect(found).To(BeTrue())
			Expect(appCreate.Routes).To(ContainElement("AppCreate"))
			Expect(appCreate.Routes).To(ContainElement("AppImportGit"))
			Expect(appCreate.Routes).To(ContainElement("AppUpload"))

			appUpdateEnv, found := auth.ActionsMap["app_update_env"]
			Expect(found).To(BeTrue())
			Expect(appUpdateEnv.Routes).To(ContainElement("EnvSet"))
			Expect(appUpdateEnv.Routes).To(ContainElement("EnvUnset"))

			appUpdateConfigs, found := auth.ActionsMap["app_update_configs"]
			Expect(found).To(BeTrue())
			Expect(appUpdateConfigs.Routes).To(ContainElement("ConfigurationBindingCreate"))
			Expect(appUpdateConfigs.Routes).To(ContainElement("ConfigurationBindingDelete"))

			appScale, found := auth.ActionsMap["app_scale"]
			Expect(found).To(BeTrue())
			Expect(appScale.Routes).To(ContainElement("AppUpdate"))

			appUpdateRoutes, found := auth.ActionsMap["app_update_routes"]
			Expect(found).To(BeTrue())
			Expect(appUpdateRoutes.Routes).To(ContainElement("AppUpdate"))

			appUpdateSettings, found := auth.ActionsMap["app_update_settings"]
			Expect(found).To(BeTrue())
			Expect(appUpdateSettings.Routes).To(ContainElement("AppUpdate"))

			appUpdateChart, found := auth.ActionsMap["app_update_chart"]
			Expect(found).To(BeTrue())
			Expect(appUpdateChart.Routes).To(ContainElement("AppUpdate"))
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
