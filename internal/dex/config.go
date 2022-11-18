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
	ID   string `yaml:"id"`
	Role string `yaml:"role"`
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
