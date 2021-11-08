package registry_test

import (
	"github.com/epinio/epinio/internal/registry"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnectionDetails", func() {
	Describe("Validate", func() {
		When("mandatory settings are empty but there are optional set", func() {
			It("returns an error", func() {
				Expect(registry.Validate("", "", "myuser", "")).To(
					MatchError("do not specify options if using the internal container registry"))
			})
		})
		When("all settings are empty", func() {
			It("returns no error", func() {
				Expect(registry.Validate("", "", "", "")).ToNot(HaveOccurred())
			})
		})
		When("mandatory settings are full and some optional are set", func() {
			It("returns no error", func() {
				Expect(registry.Validate("https://index.docker.io/v1/", "", "myuser", "")).ToNot(HaveOccurred())
			})
		})
		When("mandatory settings are full and no optional are set", func() {
			It("returns no error", func() {
				Expect(registry.Validate("https://index.docker.io/v1/", "", "", "")).ToNot(HaveOccurred())
			})
		})
	})

	Describe("PublicRegistryURL", func() {
		var details *registry.ConnectionDetails
		When("there are only localhost details", func() {
			BeforeEach(func() {
				details = &registry.ConnectionDetails{
					Namespace: "myorg",
					DockerConfigJSON: registry.DockerConfigJSON{
						Auths: map[string]registry.ContainerRegistryAuth{
							"http://127.0.0.1/": {},
						},
					},
				}
			})
			It("returns an empty string", func() {
				url, err := details.PublicRegistryURL()
				Expect(err).ToNot(HaveOccurred())
				Expect(url).To(BeEmpty())
			})
		})
		When("there is non-localhost configuration", func() {
			BeforeEach(func() {
				details = &registry.ConnectionDetails{
					Namespace: "myorg",
					DockerConfigJSON: registry.DockerConfigJSON{
						Auths: map[string]registry.ContainerRegistryAuth{
							"http://127.0.0.1/":           {},
							"https://index.docker.io/v1/": {},
						},
					},
				}
			})
			It("returns that", func() {
				url, err := details.PublicRegistryURL()
				Expect(err).ToNot(HaveOccurred())
				Expect(url).To(Equal("https://index.docker.io/v1/"))
			})
		})
	})

	Describe("PrivateRegistryURL", func() {
		var details *registry.ConnectionDetails
		When("there are non-localhost details", func() {
			BeforeEach(func() {
				details = &registry.ConnectionDetails{
					Namespace: "myorg",
					DockerConfigJSON: registry.DockerConfigJSON{
						Auths: map[string]registry.ContainerRegistryAuth{
							"https://index.docker.io/v1/": {},
						},
					},
				}
			})
			It("returns an empty string", func() {
				url, err := details.PrivateRegistryURL()
				Expect(err).ToNot(HaveOccurred())
				Expect(url).To(BeEmpty())
			})
		})
		When("there is localhost configuration", func() {
			BeforeEach(func() {
				details = &registry.ConnectionDetails{
					Namespace: "myorg",
					DockerConfigJSON: registry.DockerConfigJSON{
						Auths: map[string]registry.ContainerRegistryAuth{
							"http://127.0.0.1/":           {},
							"https://index.docker.io/v1/": {},
						},
					},
				}
			})
			It("returns that", func() {
				url, err := details.PrivateRegistryURL()
				Expect(err).ToNot(HaveOccurred())
				Expect(url).To(Equal("http://127.0.0.1/"))
			})
		})
	})
})
