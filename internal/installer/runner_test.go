package installer_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"context"

	"github.com/epinio/epinio/internal/installer"
)

type dummy struct{}

var _ installer.Action = &dummy{}

func (d dummy) Apply(ctx context.Context, c installer.Component) error {
	fmt.Printf("starting %s\n", c.ID)
	time.Sleep(2 * time.Second)
	fmt.Printf("finished %s\n", c.ID)
	return nil
}

var _ = Describe("InstallManifest", func() {
	Describe("Installing", func() {
		It("Installs all components", func() {
			m, err := installer.Load(assetPath("install-manifest.yml"))
			Expect(err).ToNot(HaveOccurred())

			plan, err := installer.BuildPlan(m.Components)
			// Plan finds no cycles
			Expect(err).ToNot(HaveOccurred())
			Expect(plan).To(HaveLen(len(m.Components)))

			// Plan doesn't know about concurrency, though
			Expect(plan.IDs()).To(Equal([]installer.DeploymentID{"linkerd", "traefik", "cert-manager", "cluster-issuers", "cluster-certificates", "tekton", "tekton-pipelines", "kubed", "epinio"}))

			// Runner doesn't need plan
			installer.Walk(context.TODO(), m.Components, &dummy{})
		})
	})
})
