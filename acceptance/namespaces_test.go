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

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Namespaces", LNamespace, func() {
	It("has a default namespace", func() {
		out, err := env.Epinio("", "namespace", "list")
		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(
			HaveATable(
				WithHeaders("NAME", "CREATED", "APPLICATIONS", "CONFIGURATIONS"),
				WithRow("workspace", WithDate(), "", ""),
			),
		)
	})

	Describe("namespace create", func() {
		var namespaceName string

		BeforeEach(func() {
			namespaceName = catalog.NewNamespaceName()
		})

		AfterEach(func() {
			env.DeleteNamespace(namespaceName)
		})

		It("creates and targets an namespace", func() {
			env.SetupAndTargetNamespace(namespaceName)
			By("switching namespace back to default")
			out, err := env.Epinio("", "target", "workspace")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Name: workspace"))
			Expect(out).To(ContainSubstring("Namespace targeted."))
		})

		It("rejects creating an existing namespace", func() {
			env.SetupAndTargetNamespace(namespaceName)
			out, err := env.Epinio("", "namespace", "create", namespaceName)
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("namespace '%s' already exists", namespaceName))
		})
	})

	Describe("namespace create failures", func() {
		It("rejects names not fitting kubernetes requirements", func() {
			namespaceName := "BOGUS"
			out, err := env.Epinio("", "namespace", "create", namespaceName)
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("name must consist of lower case alphanumeric"))
		})
	})

	Describe("namespace list", func() {
		var namespaceName string
		var configurationName string
		var appName string

		BeforeEach(func() {
			namespaceName = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespaceName)

			configurationName = catalog.NewConfigurationName()
			env.MakeConfiguration(configurationName)

			appName = catalog.NewAppName()
			out, err := env.Epinio("", "app", "create", appName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Ok"))
		})

		AfterEach(func() {
			env.DeleteNamespace(namespaceName)
		})

		It("lists namespaces", func() {
			out, err := env.Epinio("", "namespace", "list")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(
				HaveATable(
					WithHeaders("NAME", "CREATED", "APPLICATIONS", "CONFIGURATIONS"),
					WithRow(namespaceName, WithDate(), appName, configurationName),
				),
			)
		})

		It("lists namespaces in JSON format", func() {
			out, err := env.Epinio("", "namespace", "list", "--output", "json")
			Expect(err).ToNot(HaveOccurred(), out)

			namespaces := models.NamespaceList{}
			err = json.Unmarshal([]byte(out), &namespaces)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(namespaces).ToNot(BeEmpty())
		})
	})

	Describe("namespace show", func() {
		It("rejects showing an unknown namespace", func() {
			out, err := env.Epinio("", "namespace", "show", "missing-namespace")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("namespace 'missing-namespace' does not exist"))
		})

		Context("command completion", func() {
			var namespaceName string

			BeforeEach(func() {
				namespaceName = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespaceName)
			})

			AfterEach(func() {
				env.DeleteNamespace(namespaceName)
			})

			It("matches empty prefix", func() {
				out, err := env.Epinio("", "__complete", "namespace", "show", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(namespaceName))
			})

			It("does not match unknown prefix", func() {
				out, err := env.Epinio("", "__complete", "namespace", "show", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "namespace", "show", namespaceName, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(namespaceName))
			})
		})

		Context("existing namespace", func() {
			var namespaceName string
			var configurationName string
			var appName string

			BeforeEach(func() {
				namespaceName = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespaceName)

				configurationName = catalog.NewConfigurationName()
				env.MakeConfiguration(configurationName)

				appName = catalog.NewAppName()
				out, err := env.Epinio("", "app", "create", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Ok"))
			})

			AfterEach(func() {
				env.DeleteNamespace(namespaceName)
			})

			It("shows a namespace", func() {
				out, err := env.Epinio("", "namespace", "show", namespaceName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Name", namespaceName),
						WithRow("Created", WithDate()),
						WithRow("Applications", appName),
						WithRow("Configurations", configurationName),
					),
				)
			})

			It("shows a namespace in JSON format", func() {
				out, err := env.Epinio("", "namespace", "show", namespaceName, "--output", "json")
				Expect(err).ToNot(HaveOccurred(), out)

				namespace := models.Namespace{}
				err = json.Unmarshal([]byte(out), &namespace)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(namespace.Meta.Name).To(Equal(namespaceName))
			})
		})
	})

	Describe("namespace delete", func() {
		var namespaceName string

		BeforeEach(func() {
			namespaceName = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespaceName)
		})

		It("deletes an namespace", func() {
			out, err := env.Epinio("", "namespace", "delete", "-f", namespaceName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Namespaces: %s", namespaceName))
			Expect(out).To(ContainSubstring("Namespaces deleted."))
		})

		Context("command completion", func() {
			AfterEach(func() {
				env.DeleteNamespace(namespaceName)
			})

			It("matches empty prefix", func() {
				out, err := env.Epinio("", "__complete", "namespace", "delete", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(namespaceName))
			})

			It("does not match unknown prefix", func() {
				out, err := env.Epinio("", "__complete", "namespace", "delete", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "namespace", "delete", namespaceName, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(namespaceName))
			})
		})
	})

	Describe("namespace target", func() {
		It("rejects targeting an unknown namespace", func() {
			out, err := env.Epinio("", "target", "missing-namespace")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("namespace 'missing-namespace' does not exist"))
		})

		Context("existing namespace", func() {
			var namespaceName string

			BeforeEach(func() {
				namespaceName = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespaceName)
			})

			AfterEach(func() {
				env.DeleteNamespace(namespaceName)
			})

			It("shows a namespace", func() {
				out, err := env.Epinio("", "target", namespaceName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Name: %s", namespaceName))
				Expect(out).To(ContainSubstring("Namespace targeted."))
			})
		})

		Context("command completion", func() {
			var namespaceName string

			BeforeEach(func() {
				namespaceName = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespaceName)
			})

			AfterEach(func() {
				env.DeleteNamespace(namespaceName)
			})

			It("matches empty prefix", func() {
				out, err := env.Epinio("", "__complete", "target", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(namespaceName))
			})

			It("does not match unknown prefix", func() {
				out, err := env.Epinio("", "__complete", "target", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "target", namespaceName, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(namespaceName))
			})
		})
	})
})
