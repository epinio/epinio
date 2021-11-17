package admincmd_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/internal/cli/admincmd"
)

var _ = Describe("InstallManifest", func() {
	Describe("Loading", func() {
		It("loads the manifest from a file", func() {
			m, err := admincmd.Load(assetPath("install-manifest.yml"))
			Expect(err).ToNot(HaveOccurred())

			Expect(m.Components).To(HaveLen(7))

			traefik := m.Components[1]
			Expect(traefik.Type).To(Equal(admincmd.Helm))
			Expect(traefik.WaitComplete).To(HaveLen(2))
			Expect(traefik.WaitComplete[0].Type).To(Equal(admincmd.Pod))
		})
	})

	// buildPlan finds a path through the dag
	buildPlan := func(components []admincmd.Component) []admincmd.Component {
		return components
	}

	Describe("Installing", func() {
		It("Installs all components", func() {
			m, err := admincmd.Load(assetPath("install-manifest.yml"))
			Expect(err).ToNot(HaveOccurred())

			plan := buildPlan(m.Components)

			for _, c := range plan {
				fmt.Printf("installing %s with %s\n", c.ID, c.Type)
				fmt.Printf("waitComplete %#v\n", c.WaitComplete)
			}
		})
	})
})
