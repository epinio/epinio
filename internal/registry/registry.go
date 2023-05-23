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

// Package registry implements the various functions needed to store and retrieve images
// from a container registry.
package registry

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	parser "github.com/novln/docker-parser"
	"github.com/pkg/errors"
)

const (
	RegistrySecretNamespaceAnnotationKey = "epinio.io/registry-namespace" // nolint:gosec // not credentials
	CredentialsSecretName                = "registry-creds"
)

type RegistryCredentials struct {
	URL      string
	Username string
	Password string
}

type ContainerRegistryAuth struct {
	Auth     string `json:"auth"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type DockerConfigJSON struct {
	Auths map[string]ContainerRegistryAuth `json:"auths"`
}

type ConnectionDetails struct {
	RegistryCredentials []RegistryCredentials
	Namespace           string
}

// PublicRegistryURL returns the public registry URL from the connection details
// object. Assumes to have only one non-local registry in the config. If there are more,
// it will just return the first one found (no guaranteed order since there should only be
// one)
func (d *ConnectionDetails) PublicRegistryURL() (string, error) {
	r, err := regexp.Compile(`127\.0\.0\.1`)
	if err != nil {
		return "", err
	}

	for _, credentials := range d.RegistryCredentials {
		if !r.MatchString(credentials.URL) {
			return credentials.URL, nil
		}
	}

	return "", nil
}

// PrivateRegistryURL returns the internal (localhost) registry URL. That url can be used
// by Kubernetes to pull images only when the internal registry is used and exposed over
// NodePort. This method will return an empty string if no localhost URL exists in the
// config.
func (d *ConnectionDetails) PrivateRegistryURL() (string, error) {
	r, err := regexp.Compile(`127\.0\.0\.1`)
	if err != nil {
		return "", err
	}
	for _, credentials := range d.RegistryCredentials {
		if r.MatchString(credentials.URL) {
			return credentials.URL, nil
		}
	}

	return "", nil
}

// ReplaceWithInternalRegistry replaces the registry part of the given container imageURL
// with the internal (localhost) URL of the registry when the imageURL is on the Epinio
// registry (could be deploying from another registry, with the --container-image-url
// flag), and there is a localhost URL defined on the ConnectionDetails (if we are using
// an external Epinio registry, there is no need to replace anything and there is no
// localhost URL defined either).

func (d *ConnectionDetails) ReplaceWithInternalRegistry(imageURL string) (string, error) {
	privateURL, err := d.PrivateRegistryURL()
	if err != nil {
		return imageURL, err
	}
	if privateURL == "" {
		return imageURL, nil // no-op
	}

	publicURL, err := d.PublicRegistryURL()
	if err != nil {
		return imageURL, err
	}

	imageRegistryURL, _, err := ExtractImageParts(imageURL)
	if err != nil {
		return imageURL, err
	}

	if imageRegistryURL == publicURL {
		return strings.Replace(imageURL, imageRegistryURL, privateURL, -1), nil
	}

	return imageURL, nil
}

// ExtractImageParts accepts a container image URL and returns the registry
// and the image parts.
func ExtractImageParts(imageURL string) (string, string, error) {
	ref, err := parser.Parse(imageURL)
	if err != nil {
		return "", "", err
	}

	return ref.Registry(), ref.Name(), nil
}

// Validate makes sure the provided settings are valid
// The user should provide all the mandatory settings or no settings at all.
func Validate(url, namespace, username, password string) error {
	optionalSet := username != "" || password != "" || namespace != ""

	// If only optional fields are set
	if url == "" && optionalSet {
		return errors.New("do not specify options while using the internal container registry")
	}

	// Either all empty or at least the URL is set
	return nil
}

// GetConnectionDetails retrieves registry connection details from a Kubernetes secret.
func GetConnectionDetails(ctx context.Context, cluster *kubernetes.Cluster, secretNamespace, secretName string) (*ConnectionDetails, error) {
	details := ConnectionDetails{RegistryCredentials: []RegistryCredentials{}}

	secret, err := cluster.GetSecret(ctx, secretNamespace, secretName)
	if err != nil {
		return nil, err
	}

	var dockerconfigjson DockerConfigJSON
	err = json.Unmarshal(secret.Data[".dockerconfigjson"], &dockerconfigjson)
	if err != nil {
		return nil, err
	}

	details.Namespace = secret.ObjectMeta.Annotations[RegistrySecretNamespaceAnnotationKey]

	for url, auth := range dockerconfigjson.Auths {
		details.RegistryCredentials = append(details.RegistryCredentials, RegistryCredentials{
			URL:      url,
			Username: auth.Username,
			Password: auth.Password,
		})
	}

	return &details, nil
}
