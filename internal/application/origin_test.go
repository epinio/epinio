package application

import (
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Build body for SetOrigin PATCH", func() {
	Describe("Different kind of ApplicationOrigin", func() {
		var origin models.ApplicationOrigin

		When("origin is None", func() {
			BeforeEach(func() {
				origin = models.ApplicationOrigin{
					Kind: models.OriginNone,
				}
			})

			It("returns a valid JSON with no value", func() {
				body, err := buildBodyPatch(origin)

				Expect(err).ToNot(HaveOccurred())
				Expect(string(body)).To(MatchJSON(`[{"op":"replace","path":"/spec/origin","value":{"Kind":0}}]`))
			})
		})

		When("origin is Path", func() {
			BeforeEach(func() {
				origin = models.ApplicationOrigin{
					Kind: models.OriginPath,
					Path: `C:\Documents\app`,
				}
			})

			It("returns a valid JSON with path value", func() {
				body, err := buildBodyPatch(origin)

				Expect(err).ToNot(HaveOccurred())
				Expect(string(body)).To(MatchJSON(`[{"op":"replace","path":"/spec/origin","value":{"Kind":1,"path":"C:\\Documents\\app"}}]`))
			})
		})

		When("origin is Container", func() {
			BeforeEach(func() {
				origin = models.ApplicationOrigin{
					Kind:      models.OriginContainer,
					Container: "my-container",
				}
			})

			It("returns a valid JSON with container value", func() {
				body, err := buildBodyPatch(origin)

				Expect(err).ToNot(HaveOccurred())
				Expect(string(body)).To(MatchJSON(`[{"op":"replace","path":"/spec/origin","value":{"Kind":3,"container":"my-container"}}]`))
			})
		})

		When("origin is Git", func() {
			var gitOrigin, gitOriginRev models.ApplicationOrigin

			BeforeEach(func() {
				gitOrigin = models.ApplicationOrigin{
					Kind: models.OriginGit,
					Git: &models.GitRef{
						URL: "git@repo",
					},
				}

				gitOriginRev = models.ApplicationOrigin{
					Kind: models.OriginGit,
					Git: &models.GitRef{
						URL:      "git@repo",
						Revision: "revision_1",
					},
				}
			})

			Context("with no URL value", func() {
				It("returns a valid JSON with git value with repo", func() {
					body, err := buildBodyPatch(gitOrigin)

					Expect(err).ToNot(HaveOccurred())
					Expect(string(body)).To(MatchJSON(`[{"op":"replace","path":"/spec/origin","value":{"Kind":2,"git":{"repository":"git@repo"}}}]`))
				})
			})

			Context("with URL value", func() {
				It("returns a valid JSON with git value with repo and revision", func() {
					body, err := buildBodyPatch(gitOriginRev)

					Expect(err).ToNot(HaveOccurred())
					Expect(string(body)).To(MatchJSON(`[{"op":"replace","path":"/spec/origin","value":{"Kind":2,"git":{"repository":"git@repo","revision":"revision_1"}}}]`))
				})
			})
		})
	})
})
