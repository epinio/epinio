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

package auth_test

import (
	"github.com/epinio/epinio/internal/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type namespacedFake struct {
	namespace string
	name      string
}

func (f namespacedFake) Namespace() string {
	return f.namespace
}

var _ = Describe("FilterByNamespaces", func() {
	var resources []namespacedFake

	BeforeEach(func() {
		resources = []namespacedFake{
			{namespace: "alpha", name: "one"},
			{namespace: "beta", name: "two"},
			{namespace: "alpha", name: "three"},
			{namespace: "gamma", name: "four"},
		}
	})

	When("the namespaces list is empty", func() {
		It("returns the resources untouched for a nil list", func() {
			filtered := auth.FilterByNamespaces(resources, nil)

			Expect(filtered).To(Equal(resources))
		})

		It("returns the resources untouched for an empty list", func() {
			filtered := auth.FilterByNamespaces(resources, []string{})

			Expect(filtered).To(Equal(resources))
		})
	})

	When("one namespace is requested", func() {
		It("keeps every resource in that namespace", func() {
			filtered := auth.FilterByNamespaces(resources, []string{"alpha"})

			Expect(filtered).To(Equal([]namespacedFake{
				{namespace: "alpha", name: "one"},
				{namespace: "alpha", name: "three"},
			}))
		})
	})

	When("several namespaces are requested", func() {
		It("keeps the union and preserves input order", func() {
			requested := []string{"gamma", "beta"}

			filtered := auth.FilterByNamespaces(resources, requested)

			Expect(filtered).To(Equal([]namespacedFake{
				{namespace: "beta", name: "two"},
				{namespace: "gamma", name: "four"},
			}))
		})

		It("ignores a duplicate namespace", func() {
			requested := []string{"alpha", "alpha"}

			filtered := auth.FilterByNamespaces(resources, requested)

			Expect(filtered).To(HaveLen(2))
		})
	})

	When("a requested namespace matches nothing", func() {
		It("drops the unknown name and honours the rest", func() {
			requested := []string{"beta", "does-not-exist"}

			filtered := auth.FilterByNamespaces(resources, requested)

			Expect(filtered).To(Equal([]namespacedFake{
				{namespace: "beta", name: "two"},
			}))
		})

		// Empty but not nil: the non-paginated handlers return this slice
		// straight to the client, and nil marshals to JSON null.
		It("returns an empty non-nil slice when nothing matches", func() {
			requested := []string{"does-not-exist"}

			filtered := auth.FilterByNamespaces(resources, requested)

			Expect(filtered).To(BeEmpty())
			Expect(filtered).NotTo(BeNil())
		})
	})
})
