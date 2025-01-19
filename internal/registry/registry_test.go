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

package registry_test

import (
	"fmt"

	"github.com/epinio/epinio/internal/registry"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnectionDetails", func() {
	Describe("Validate", func() {
		When("mandatory settings are empty but there are optional set", func() {
			It("returns an error", func() {
				Expect(registry.Validate("", "", "myuser", "")).To(
					MatchError("do not specify options while using the internal container registry"))
			})
		})
		When("all settings are empty", func() {
			It("returns no error", func() {
				Expect(registry.Validate("", "", "", "")).ToNot(HaveOccurred())
			})
		})
		When("mandatory settings are full and some optional are set", func() {
			It("returns no error", func() {
				Expect(registry.Validate("registry.hub.docker.com", "", "myuser", "")).ToNot(HaveOccurred())
			})
		})
		When("mandatory settings are full and no optional are set", func() {
			It("returns no error", func() {
				Expect(registry.Validate("registry.hub.docker.com", "", "", "")).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Validate", func() {
		var imageURL string
		BeforeEach(func() {
			imageURL = "epinio/sample-app"
		})
		It("extracts the registry and image parts", func() {
			registry, image, err := registry.ExtractImageParts(imageURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(registry).To(Equal("docker.io"))
			Expect(image).To(Equal("epinio/sample-app:latest"))
		})
	})

	Describe("PublicRegistryURL", func() {
		var details *registry.ConnectionDetails
		When("there are only localhost details", func() {
			BeforeEach(func() {
				details = &registry.ConnectionDetails{
					Namespace: "myorg",
					RegistryCredentials: []registry.RegistryCredentials{
						{URL: "http://127.0.0.1/"},
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
					RegistryCredentials: []registry.RegistryCredentials{
						{URL: "http://127.0.0.1/"},
						{URL: "registry.hub.docker.com"},
					},
				}
			})
			It("returns that", func() {
				url, err := details.PublicRegistryURL()
				Expect(err).ToNot(HaveOccurred())
				Expect(url).To(Equal("registry.hub.docker.com"))
			})
		})
	})

	Describe("PrivateRegistryURL", func() {
		var details *registry.ConnectionDetails
		When("there are non-localhost details", func() {
			BeforeEach(func() {
				details = &registry.ConnectionDetails{
					Namespace: "myorg",
					RegistryCredentials: []registry.RegistryCredentials{
						{URL: "registry.hub.docker.com"},
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
					RegistryCredentials: []registry.RegistryCredentials{
						{URL: "http://127.0.0.1/"},
						{URL: "registry.hub.docker.com"},
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

	Describe("ReplaceWithInternalRegistry", func() {
		var details *registry.ConnectionDetails
		var imageURL string
		When("there are non-localhost details", func() {
			BeforeEach(func() {
				details = &registry.ConnectionDetails{
					Namespace: "myorg",
					RegistryCredentials: []registry.RegistryCredentials{
						{URL: "registry.hub.docker.com"},
					},
				}
				imageURL = "epinio/my-app"
			})
			It("returns the image url unchanged", func() {
				newImageURL, err := details.ReplaceWithInternalRegistry(imageURL)
				Expect(err).ToNot(HaveOccurred())
				Expect(newImageURL).To(Equal(imageURL))
			})
		})
		When("there is a localhost registry url", func() {
			var publicRegistryURL string
			BeforeEach(func() {
				publicRegistryURL = fmt.Sprintf("%s.%s", "epinio-registry", "1.2.3.4.sslip.io")
			})
			When("the image url matches the public registry URL", func() {
				BeforeEach(func() {
					details = &registry.ConnectionDetails{
						Namespace: "myorg",
						RegistryCredentials: []registry.RegistryCredentials{
							{URL: publicRegistryURL},
							{URL: "127.0.0.1:30500"},
						},
					}
					imageURL = publicRegistryURL + "/apps/my-app"
				})
				It("replaces the registry part with the internal URL", func() {
					newImageURL, err := details.ReplaceWithInternalRegistry(imageURL)
					Expect(err).ToNot(HaveOccurred())
					Expect(newImageURL).To(Equal("127.0.0.1:30500/apps/my-app"))
				})
			})
			When("the image url doesn't match the public registry URL", func() {
				BeforeEach(func() {
					details = &registry.ConnectionDetails{
						Namespace: "myorg",
						RegistryCredentials: []registry.RegistryCredentials{
							{URL: "registry.hub.docker.com"},
							{URL: "127.0.0.1:30500"},
						},
					}
					imageURL = "otherregistry.com/apps/my-app"
				})
				It("leaves the image URL unchanged", func() {
					newImageURL, err := details.ReplaceWithInternalRegistry(imageURL)
					Expect(err).ToNot(HaveOccurred())
					Expect(newImageURL).To(Equal(imageURL))
				})
			})
		})
	})
})
