package installer_test

import (
	"github.com/epinio/epinio/internal/installer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manifest", func() {
	var manifest installer.Manifest
	Describe("Validate", func() {
		When("manifest has circular dependencies", func() {
			BeforeEach(func() {
				components := installer.Components{
					{ID: "component1", Needs: []string{"component2"}},
					{ID: "component2", Needs: []string{"component3"}},
					{ID: "component3", Needs: []string{"component1"}},
				}

				manifest = installer.Manifest{
					Components: components,
				}
			})

			It("fails", func() {
				err := manifest.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(MatchRegexp("Circular dependency"))
			})
		})

		When("manifest has no circular dependencies", func() {
			BeforeEach(func() {
				components := installer.Components{
					{ID: "component1", Needs: []string{}},
					{ID: "component2", Needs: []string{}},
					{ID: "component3", Needs: []string{"component1"}},
					{ID: "component4", Needs: []string{"component2"}},
				}

				manifest = installer.Manifest{
					Components: components,
				}
			})

			It("succeeds", func() {
				err := manifest.Validate()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
