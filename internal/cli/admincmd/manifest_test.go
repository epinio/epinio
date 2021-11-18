package admincmd_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/internal/cli/admincmd"
)

var _ = Describe("InstallManifest", func() {
	Describe("Loading", func() {
		It("loads the manifest from a file", func() {
			m, err := admincmd.Load(assetPath("install-manifest.yml"))
			Expect(err).ToNot(HaveOccurred())

			Expect(m.Components).To(HaveLen(9))

			traefik := m.Components[1]
			Expect(traefik.Type).To(Equal(admincmd.Helm))
			Expect(traefik.WaitComplete).To(HaveLen(2))
			Expect(traefik.WaitComplete[0].Type).To(Equal(admincmd.Pod))
		})
	})

	Describe("Installing", func() {
		It("Installs all components", func() {
			m, err := admincmd.Load(assetPath("install-manifest.yml"))
			Expect(err).ToNot(HaveOccurred())

			plan, err := admincmd.BuildPlan(m.Components)
			// Plan finds no cycles
			Expect(err).ToNot(HaveOccurred())
			Expect(plan).To(HaveLen(len(m.Components)))

			// Plan doesn't know about concurrency, though
			Expect(plan.IDs()).To(Equal([]admincmd.DeploymentID{"linkerd", "traefik", "cert-manager", "cluster-issuers", "cluster-certificates", "tekton", "tekton-pipelines", "kubed", "epinio"}))

			// Runner doesn't need plan
			err = admincmd.Runner(m.Components)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
