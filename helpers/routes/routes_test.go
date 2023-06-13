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

package routes_test

import (
	"github.com/epinio/epinio/helpers/routes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Routes", func() {

	Describe("Path", func() {
		var nr routes.NamedRoutes

		BeforeEach(func() {
			nr = routes.NamedRoutes{}
			nr["a"] = routes.NewRoute("get", "foo", nil)
			nr["b"] = routes.NewRoute("post", "foo/%s", nil)
		})

		It("returns a proper path for known route without params", func() {
			s := nr.Path("a")
			Expect(s).To(Equal("foo"))
		})

		It("returns a proper path for known route with params", func() {
			s := nr.Path("b", "x")
			Expect(s).To(Equal("foo/x"))
		})

		It("returns a bad path for a known route expecting params and not given any", func() {
			s := nr.Path("b")
			Expect(s).To(Equal("foo/%s"))
		})

		It("panics for an unknown route", func() {
			unknownRoute := func() {
				_ = nr.Path("c")
			}
			Expect(unknownRoute).To(PanicWith("route not found for 'c'"))
		})
	})
})
