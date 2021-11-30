package installer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/internal/installer"
)

var _ = Describe("InstallManifest", func() {
	Describe("Loading", func() {
		It("loads the manifest from a file", func() {
			m, err := installer.Load(assetPath("install-manifest.yml"))
			Expect(err).ToNot(HaveOccurred())

			Expect(m.Components).To(HaveLen(10))

			traefik := m.Components[2]
			Expect(traefik.Type).To(Equal(installer.Helm))
			Expect(traefik.WaitComplete).To(HaveLen(2))
			Expect(traefik.WaitComplete[0].Type).To(Equal(installer.Pod))
		})
	})
})
