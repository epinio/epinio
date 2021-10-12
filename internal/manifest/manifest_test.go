package manifest_test

import (
	"os"

	"github.com/epinio/epinio/internal/manifest"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manifest", func() {
	Describe("Get", func() {
		When("the desired manifest file is missing", func() {
			var workdir string
			BeforeEach(func() {
				var err error
				workdir, err = os.Getwd()
				Expect(err).ToNot(HaveOccurred(), workdir)
			})
			It("returns defaults", func() {
				m, err := manifest.Get("bogus.yml")
				Expect(err).ToNot(HaveOccurred())
				Expect(m.Name).To(Equal(""))
				Expect(m.Configuration.Services).To(BeNil())
				Expect(m.Configuration.Instances).To(BeNil())
				Expect(m.Configuration.Environment).To(BeNil())
				Expect(m.Self).To(Equal("<<Defaults>>"))
				Expect(m.Origin.Kind).To(Equal(models.OriginPath))
				Expect(m.Origin.Container).To(Equal(""))
				Expect(m.Origin.Git.Revision).To(Equal(""))
				Expect(m.Origin.Git.URL).To(Equal(""))
				Expect(m.Origin.Path).To(Equal(workdir))
				Expect(m.Staging.Builder).To(Equal(manifest.DefaultBuilder))
			})
		})

		// TODO: No permission to access existing manifest file.
		// TODO: Bad YAML.
		// TODO: Good YAML.
	})
})
