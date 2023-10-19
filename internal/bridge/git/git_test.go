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

package git_test

import (
	"context"
	"math/rand"
	"time"

	"github.com/epinio/epinio/internal/bridge/git"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mockSecretLister struct {
	list *v1.SecretList
	err  error
}

func (m *mockSecretLister) List(ctx context.Context, opts metav1.ListOptions) (*v1.SecretList, error) {
	return m.list, m.err
}

var r *rand.Rand

var _ = Describe("Manager", func() {

	BeforeEach(func() {
		r = rand.New(rand.NewSource(time.Now().UnixNano()))
	})

	Describe("NewManager", func() {
		When("multiple secrets are found", func() {
			It("returns the configurations in the same order", func() {
				secrets := []v1.Secret{
					{ObjectMeta: metav1.ObjectMeta{Name: "ccc"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "aaa"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "ddd"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "bbb"}},
				}

				r.Shuffle(len(secrets), func(i, j int) {
					secrets[i], secrets[j] = secrets[j], secrets[i]
				})

				mockSecretLister := &mockSecretLister{list: &v1.SecretList{Items: secrets}}

				gitManager, err := git.NewManager(logr.Discard(), mockSecretLister)
				Expect(err).To(BeNil())

				Expect(gitManager.Configurations[0].ID).To(Equal("aaa"))
				Expect(gitManager.Configurations[1].ID).To(Equal("bbb"))
				Expect(gitManager.Configurations[2].ID).To(Equal("ccc"))
				Expect(gitManager.Configurations[3].ID).To(Equal("ddd"))
			})
		})
	})

	Describe("FindConfiguration", func() {
		When("configurations are empty", func() {
			It("returns no configuration", func() {
				manager := &git.Manager{Configurations: []git.Configuration{}}

				config, err := manager.FindConfiguration("https://github.com/username/repo")
				Expect(config).To(BeNil())
				Expect(err).To(BeNil())
			})
		})

		When("configurations are not matching", func() {
			It("returns no configuration", func() {
				configs := []git.Configuration{
					newConfiguration("another", "https://another.com", "", ""),
					newConfiguration("gitlab", "https://gitlab.com", "", ""),
					newConfiguration("github-username-repo2", "https://github.com", "username", "repo2"),
					newConfiguration("github-username2", "https://github.com", "username2", ""),
				}

				manager := &git.Manager{Configurations: configs}

				config, err := manager.FindConfiguration("https://github.com/username/repo")
				Expect(config).To(BeNil())
				Expect(err).To(BeNil())
			})
		})

		When("a configuration for a whole provider is matching", func() {
			It("returns the provider configuration", func() {
				configs := []git.Configuration{
					newConfiguration("another", "https://another.com", "", ""),
					newConfiguration("gitlab", "https://gitlab.com", "", ""),
					newConfiguration("github", "https://github.com", "", ""),
					newConfiguration("github-username-repo2", "https://github.com", "username", "repo2"),
					newConfiguration("github-username2", "https://github.com", "username2", ""),
				}

				manager := &git.Manager{Configurations: configs}

				config, err := manager.FindConfiguration("https://github.com/username/repo")
				Expect(config).ToNot(BeNil())
				Expect(err).To(BeNil())
				Expect(config.ID).To(Equal("github"))
			})
		})

		When("a configuration for the org is matching", func() {
			It("returns the org configuration", func() {
				configs := []git.Configuration{
					newConfiguration("another", "https://another.com", "", ""),
					newConfiguration("gitlab", "https://gitlab.com", "", ""),
					newConfiguration("github", "https://github.com", "", ""),
					newConfiguration("github-username-repo2", "https://github.com", "username", "repo2"),
					newConfiguration("github-username", "https://github.com", "username", ""),
					newConfiguration("github-username2", "https://github.com", "username2", ""),
				}

				manager := &git.Manager{Configurations: configs}

				config, err := manager.FindConfiguration("https://github.com/username/repo")
				Expect(config).ToNot(BeNil())
				Expect(err).To(BeNil())
				Expect(config.ID).To(Equal("github-username"))
			})
		})

		When("a configuration for a repo exists", func() {
			It("returns the specific configuration", func() {
				configs := []git.Configuration{
					newConfiguration("another", "https://another.com", "", ""),
					newConfiguration("gitlab", "https://gitlab.com", "", ""),
					newConfiguration("github", "https://github.com", "", ""),
					newConfiguration("github-username-repo", "https://github.com", "username", "repo"),
					newConfiguration("github-username-repo2", "https://github.com", "username", "repo2"),
					newConfiguration("github-username", "https://github.com", "username", ""),
					newConfiguration("github-username2", "https://github.com", "username2", ""),
				}

				manager := &git.Manager{Configurations: configs}

				config, err := manager.FindConfiguration("https://github.com/username/repo2")
				Expect(config).ToNot(BeNil())
				Expect(err).To(BeNil())
				Expect(config.ID).To(Equal("github-username-repo2"))
			})
		})

		When("multiple org and repo configurations matches", func() {
			It("returns the most specific one", func() {
				configs := []git.Configuration{
					newConfiguration("another", "https://another.com", "", ""),
					newConfiguration("gitlab", "https://gitlab.com", "", ""),
					newConfiguration("github", "https://github.com", "", ""),
					newConfiguration("github-username-repo", "https://github.com", "username", "repo"),
					newConfiguration("github-username-repo2", "https://github.com", "username", "repo2"),
					newConfiguration("github-username", "https://github.com", "username", ""),
					newConfiguration("github-username2", "https://github.com", "username2", ""),
				}

				manager := &git.Manager{Configurations: configs}

				config, err := manager.FindConfiguration("https://github.com/username/repo2")
				Expect(config).ToNot(BeNil())
				Expect(err).To(BeNil())
				Expect(config.ID).To(Equal("github-username-repo2"))
			})
		})

		When("multiple specific configurations matches", func() {
			It("returns the first one, ordered by ID", func() {
				configs := []git.Configuration{
					newConfiguration("github-username-repo-1", "https://github.com", "username", "repo"),
					newConfiguration("github-username-repo-2", "https://github.com", "username", "repo"),
					newConfiguration("github-username-repo-3", "https://github.com", "username", "repo"),
				}

				manager := &git.Manager{Configurations: configs}

				config, err := manager.FindConfiguration("https://github.com/username/repo")
				Expect(config).ToNot(BeNil())
				Expect(err).To(BeNil())
				Expect(config.ID).To(Equal("github-username-repo-1"))
			})
		})
	})

	Describe("NewSecretFromConfiguration", func() {
		When("some secrets exists", func() {
			It("returns the expected configurations", func() {
				configs := git.NewConfigurationsFromSecrets([]v1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "my-config"},
						Data: map[string][]byte{
							"url":         []byte("giturl"),
							"provider":    []byte("github"),
							"username":    []byte("myuser"),
							"password":    []byte("mypass"),
							"userOrg":     []byte("myuserorg"),
							"repo":        []byte("myrepo"),
							"skipSSL":     []byte("true"),
							"certificate": []byte("----CERT----"),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "my-config-skipssl-false"},
						Data: map[string][]byte{
							"skipSSL": []byte("false"),
						},
					},

					{
						ObjectMeta: metav1.ObjectMeta{Name: "my-config-skipssl-anything"},
						Data: map[string][]byte{
							"skipSSL": []byte("anything"),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "my-config-provider-unknown"},
						Data: map[string][]byte{
							"provider": []byte("sdjnksd"),
						},
					},
				})

				Expect(configs).ToNot(BeNil())
				Expect(configs).To(HaveLen(4))

				Expect(configs[0].ID).To(Equal("my-config"))
				Expect(configs[0].Provider).To(Equal(models.ProviderGithub))
				Expect(configs[0].Username).To(Equal("myuser"))
				Expect(configs[0].Password).To(Equal("mypass"))
				Expect(configs[0].UserOrg).To(Equal("myuserorg"))
				Expect(configs[0].Repository).To(Equal("myrepo"))
				Expect(configs[0].SkipSSL).To(BeTrue())
				Expect(configs[0].Certificate).To(Equal([]byte("----CERT----")))

				Expect(configs[1].ID).To(Equal("my-config-skipssl-false"))
				Expect(configs[1].SkipSSL).To(BeFalse())

				Expect(configs[2].ID).To(Equal("my-config-skipssl-anything"))
				Expect(configs[2].SkipSSL).To(BeFalse())

				Expect(configs[3].ID).To(Equal("my-config-provider-unknown"))
				Expect(configs[3].Provider).To(Equal(models.ProviderUnknown))
			})
		})
	})

	Describe("NewSecretFromConfiguration and NewSecretFromConfiguration", func() {
		When("some secrets exists and they are encoded to configs", func() {
			It("will return the same secrets", func() {
				originalSecrets := []v1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "my-config"},
						Data: map[string][]byte{
							"url":         []byte("giturl"),
							"provider":    []byte("github"),
							"username":    []byte("myuser"),
							"password":    []byte("mypass"),
							"userOrg":     []byte("myuserorg"),
							"repo":        []byte("myrepo"),
							"skipSSL":     []byte("true"),
							"certificate": []byte("----CERT----"),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "my-config-skipssl-false"},
						Data: map[string][]byte{
							"skipSSL": []byte("false"),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "my-config-skipssl-anything"},
						Data: map[string][]byte{
							"skipSSL": []byte("anything"),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "my-config-provider-unknown"},
						Data: map[string][]byte{
							"provider": []byte("sdjnksd"),
						},
					},
				}

				configs := git.NewConfigurationsFromSecrets(originalSecrets)

				secrets := []v1.Secret{}
				for _, conf := range configs {
					secrets = append(secrets, git.NewSecretFromConfiguration(conf))
				}

				Expect(secrets).ToNot(BeNil())
				Expect(secrets).To(HaveLen(4))

				Expect(secrets[0].Data).To(Equal(originalSecrets[0].Data))
				Expect(secrets[1].Data).To(Equal(map[string][]byte{
					"provider": []byte("unknown"),
					"skipSSL":  []byte("false"),
				}))
				Expect(secrets[2].Data).To(Equal(map[string][]byte{
					"provider": []byte("unknown"),
					"skipSSL":  []byte("false"),
				}))
				Expect(secrets[3].Data).To(Equal(map[string][]byte{
					"provider": []byte("unknown"),
					"skipSSL":  []byte("false"),
				}))
			})
		})
	})
})

func newConfiguration(ID, url, userOrg, repo string) git.Configuration {
	return git.Configuration{
		ID:         ID,
		URL:        url,
		UserOrg:    userOrg,
		Repository: repo,
	}
}
