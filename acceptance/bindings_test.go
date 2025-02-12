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
	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bounds between Apps & Configurations", LApplication, func() {
	var namespace string
	containerImageURL := "epinio/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	Describe("Display", func() {
		var appName string
		var configurationName string

		BeforeEach(func() {
			appName = catalog.NewAppName()
			configurationName = catalog.NewConfigurationName()

			env.MakeContainerImageApp(appName, 1, containerImageURL)
			env.MakeConfiguration(configurationName)
			env.BindAppConfiguration(appName, configurationName, namespace)
		})

		AfterEach(func() {
			// Delete app first, as this also unbinds the configuration
			env.CleanupApp(appName)
			env.CleanupConfiguration(configurationName)
		})

		It("shows the bound app for configurations list, and vice versa", func() {
			out, err := env.Epinio("", "configuration", "list")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(
				HaveATable(
					WithHeaders("NAME", "CREATED", "TYPE", "ORIGIN", "APPLICATIONS"),
					WithRow(configurationName, WithDate(), "custom", "", appName),
				),
			)

			// The next check uses `Eventually` because binding the
			// configuration to the app forces a restart of the app's
			// pod. It takes the system some time to terminate the
			// old pod, and spin up the new, during which `app list`
			// will return inconsistent results about the desired
			// and actual number of instances. We wait for the
			// system to settle back into a normal state.

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "5m").Should(
				HaveATable(
					WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
					WithRow(appName, WithDate(), "1/1", appName+".*", configurationName, ""),
				),
			)
		})
	})
})
