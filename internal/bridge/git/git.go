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

package git

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type SecretLister interface {
	List(ctx context.Context, opts metav1.ListOptions) (*v1.SecretList, error)
}

type Manager struct {
	logger         logr.Logger
	SecretLister   SecretLister
	Configurations []Configuration
}

// Configuration is used to customize the Git requests for a  specific git provider.
// The only required field is the URL, needed to check the specific instance to apply the configuration.
// If the UserOrg and/or the Repository are also specified then the most specific configuration will be used.
type Configuration struct {
	// TODO : Track creating user

	// ID of the configuration (it maps to the kubernetes secret)
	ID string
	// URL is the full url (schema/host/port) used to match a particular instance
	URL      string
	Provider models.GitProvider
	// Username and Password are used to perform a BasicAuth.
	// For Github/Gitlab the username can be anything (see https://gitlab.com/gitlab-org/gitlab/-/issues/212953).
	Username string
	// The Personal Access Token
	Password string
	// UserOrg is used to specify the username/organization/project
	UserOrg string
	// Repository is used to specify the exact repository
	Repository  string
	SkipSSL     bool
	Certificate []byte
}

// Gitconfig returns the id of the configuration, for filtering.
// Satisfies the interface `GitconfigResource`, see package `internal/auth`
func (c Configuration) Gitconfig() string {
	return c.ID
}

func NewManager(logger logr.Logger, secretLoader SecretLister) (*Manager, error) {
	logger = logger.WithName("GitManager")

	secretSelector := labels.Set(map[string]string{
		kubernetes.EpinioAPIGitCredentialsLabelKey: "true",
	}).AsSelector().String()

	secretList, err := secretLoader.List(context.Background(), metav1.ListOptions{LabelSelector: secretSelector})
	if err != nil {
		return nil, err
	}
	configurations := NewConfigurationsFromSecrets(secretList.Items)

	configIDs := []string{}
	for _, config := range configurations {
		configIDs = append(configIDs, config.ID)
	}

	// Sort the configurations to fetch always the same, in case of multiple matches.
	// This will help in case of errors because in case of multiple configurations always the same will be chosen.
	// Having instead a different one everytime could cause sporadic errors.
	sort.Slice(configurations, func(i, j int) bool {
		return configurations[i].ID < configurations[j].ID
	})

	logger.V(1).Info(fmt.Sprintf("found %d git configurations [%s]", len(configurations), strings.Join(configIDs, ", ")))

	return &Manager{
		logger:         logger,
		SecretLister:   secretLoader,
		Configurations: configurations,
	}, nil
}

func NewConfigurationsFromSecrets(secrets []v1.Secret) []Configuration {
	configs := make([]Configuration, 0, len(secrets))

	for _, sec := range secrets {
		config := &Configuration{
			ID:          string(sec.Name),
			URL:         string(sec.Data["url"]),
			Username:    string(sec.Data["username"]),
			Password:    string(sec.Data["password"]),
			UserOrg:     string(sec.Data["userOrg"]),
			Repository:  string(sec.Data["repo"]),
			Certificate: sec.Data["certificate"],
		}

		// the func is always returning a provider, if err a models.ProviderUnknown
		config.Provider, _ = models.GitProviderFromString(string(sec.Data["provider"]))

		skipSSLVal, found := sec.Data["skipSSL"]
		// if not found skipSSL is false, otherwise it needs to be "true"
		if found && strings.ToLower(string(skipSSLVal)) == "true" {
			config.SkipSSL = true
		}

		configs = append(configs, *config)
	}

	return configs
}

func NewSecretFromConfiguration(config Configuration) v1.Secret {
	// helper to set the value in the map, if present
	setValue := func(m map[string][]byte, key, value string) map[string][]byte {
		if len(value) > 0 {
			m[key] = []byte(value)
		}
		return m
	}

	dataMap := make(map[string][]byte)

	dataMap = setValue(dataMap, "url", string(config.URL))
	dataMap = setValue(dataMap, "provider", string(config.Provider))
	dataMap = setValue(dataMap, "username", string(config.Username))
	dataMap = setValue(dataMap, "password", string(config.Password))
	dataMap = setValue(dataMap, "userOrg", string(config.UserOrg))
	dataMap = setValue(dataMap, "repo", string(config.Repository))
	dataMap = setValue(dataMap, "certificate", string(config.Certificate))

	if config.SkipSSL {
		dataMap = setValue(dataMap, "skipSSL", "true")
	} else {
		dataMap = setValue(dataMap, "skipSSL", "false")
	}

	return v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ID,
			Namespace: helmchart.Namespace(),
			Labels: map[string]string{
				kubernetes.EpinioAPIGitCredentialsLabelKey: "true",
			},
		},
		Data: dataMap,
	}
}

// FindConfiguration will load the most specific Configuration for the provided gitUrl.
// A gitURL is a full git repo url like 'https://github.com/username/repo'
func (m *Manager) FindConfiguration(gitURL string) (*Configuration, error) {

	// We could have multiple configurations that are valid for a specific provider
	// and we need to find the most specific one.
	availableConfigurations := map[string][]Configuration{}

	gitInfo, err := newGitRepoInfoFromURL(gitURL)
	if err != nil {
		return nil, err
	}

	for _, config := range m.Configurations {
		// create the key for this config
		configMapKey := config.URL
		if config.UserOrg != "" {
			configMapKey = fmt.Sprintf("%s/%s", config.URL, config.UserOrg)
		}
		if config.Repository != "" {
			configMapKey = fmt.Sprintf("%s/%s/%s", config.URL, config.UserOrg, config.Repository)
		}

		// append the config to the available configurations
		configsForKey, found := availableConfigurations[configMapKey]
		if !found {
			configsForKey = []Configuration{}
		}
		availableConfigurations[configMapKey] = append(configsForKey, config)
	}

	// now we need to check if there is a configuration available for the specific repo,
	// or for the userOrg, or at least for the provider.
	// I.e. if we have a config for the 'https://github.com/username/repo' we should return it,
	// otherwise we could return the more generic config for the 'https://github.com/username'
	// or the global 'https://github.com' configuration

	toMatch := fmt.Sprintf("%s/%s/%s", gitInfo.URL, gitInfo.UserOrg, gitInfo.Repository)
	if userOrgRepoConfigs, found := availableConfigurations[toMatch]; found {
		return &userOrgRepoConfigs[0], nil
	}

	toMatch = fmt.Sprintf("%s/%s", gitInfo.URL, gitInfo.UserOrg)
	if userOrgRepoConfigs, found := availableConfigurations[toMatch]; found {
		return &userOrgRepoConfigs[0], nil
	}

	if userOrgRepoConfigs, found := availableConfigurations[gitInfo.URL]; found {
		return &userOrgRepoConfigs[0], nil
	}

	return nil, nil
}

type gitRepoInfo struct {
	URL        string
	UserOrg    string
	Repository string
}

// newGitRepoInfoFromURL will create a gitRepoInfo from the full git URL
func newGitRepoInfoFromURL(gitURL string) (*gitRepoInfo, error) {
	url, err := url.Parse(gitURL)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing gitURL [%s]", gitURL)
	}

	// gitRepoHost has the host of the gitURL, i.e.: https://github.com
	gitRepoHost := fmt.Sprintf("%s://%s", url.Scheme, url.Host)

	path := strings.TrimPrefix(url.Path, "/")
	orgRepoPath := strings.Split(path, "/")

	info := &gitRepoInfo{
		URL:     gitRepoHost,
		UserOrg: orgRepoPath[0],
	}

	if len(orgRepoPath) > 1 {
		info.Repository = orgRepoPath[1]
	}

	return info, nil
}
