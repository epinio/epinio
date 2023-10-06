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

package dex

import (
	"net/url"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Issuer          string
	ClientID        string
	Endpoint        *url.URL
	ProvidersGroups []ProviderGroups

	Oauth2 *oauth2.Config
}

type ProviderGroups struct {
	ConnectorID string  `yaml:"connectorId"`
	Groups      []Group `yaml:"groups"`
}

type Group struct {
	ID    string   `yaml:"id"`
	Role  string   `yaml:"role"`
	Roles []string `yaml:"roles"`
}

func NewConfig(issuer, clientID string) (Config, error) {
	config := Config{
		Issuer:   issuer,
		ClientID: clientID,
	}

	endpoint, err := url.Parse(issuer)
	if err != nil {
		return config, errors.Wrap(err, "parsing the issuer URL")
	}
	config.Endpoint = endpoint

	return config, nil
}

func NewConfigFromSecretData(clientID string, secretData map[string][]byte) (Config, error) {
	config := Config{
		ClientID: clientID,
	}

	issuer, found := secretData["issuer"]
	if found {
		config.Issuer = string(issuer)
	}

	endpoint, found := secretData["endpoint"]
	if found {
		endpointURL, err := url.Parse(string(endpoint))
		if err != nil {
			return config, errors.Wrap(err, "parsing the issuer URL")
		}
		config.Endpoint = endpointURL
	}

	rolesMapping, found := secretData["rolesMapping"]
	if found {
		providersGroups := []ProviderGroups{}

		err := yaml.Unmarshal(rolesMapping, &providersGroups)
		if err != nil {
			return config, errors.Wrap(err, "parsing the issuer URL")
		}

		config.ProvidersGroups = providersGroups
	}

	return config, nil
}
