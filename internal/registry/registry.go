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
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/bridge/git"
	parser "github.com/novln/docker-parser"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	RegistrySecretNamespaceAnnotationKey = "epinio.io/registry-namespace" // nolint:gosec // not credentials
	CredentialsSecretName                = "registry-creds"
)

type RegistryCredentials struct {
	URL      string
	Username string
	Password string // nolint:gosec // intentional auth field for registry
}

type ContainerRegistryAuth struct {
	Auth     string `json:"auth"`
	Username string `json:"username"`
	Password string `json:"password"` // nolint:gosec // intentional auth field for registry
}

type DockerConfigJSON struct {
	Auths map[string]ContainerRegistryAuth `json:"auths"`
}

type ConnectionDetails struct {
	RegistryCredentials []RegistryCredentials
	Namespace           string
}

type ExportRegistry struct {
	Name string
	URL  string
}

func ExportRegistryNames(secretLoader git.SecretLister) ([]string, error) {

	registries, err := ExportRegistries(secretLoader)
	if err != nil {
		return nil, err
	}

	names := []string{}
	for _, registry := range registries {
		names = append(names, registry.Name)
	}

	return names, nil
}

func ExportRegistries(secretLoader git.SecretLister) ([]ExportRegistry, error) {

	secretSelector := labels.Set(map[string]string{
		kubernetes.EpinioAPIExportRegistryLabelKey: "true",
	}).AsSelector().String()

	secretList, err := secretLoader.List(context.Background(),
		metav1.ListOptions{
			LabelSelector: secretSelector,
		})
	if err != nil {
		return nil, err
	}

	registries := []ExportRegistry{}
	for _, secret := range secretList.Items {
		url, err := GetRegistryUrlFromSecret(secret)
		if err != nil {
			// log the issue, and otherwise ignore the secret
			helpers.Logger.Errorw("skipping secret", "error", err, "secret", secret.Name)
			continue
		}

		registries = append(registries, ExportRegistry{
			Name: string(secret.Name),
			URL:  url,
		})
	}

	return registries, nil
}

func GetRegistryUrlFromSecret(secret v1.Secret) (string, error) {
	namespace, ok := secret.Annotations[RegistrySecretNamespaceAnnotationKey]
	if !ok {
		return "", fmt.Errorf("missing annotation '%s'", RegistrySecretNamespaceAnnotationKey)
	}

	configjson, ok := secret.Data[".dockerconfigjson"]
	if !ok {
		return "", errors.New("missing key `.dockerconfigjson`")
	}

	// The JSON has to have the following structure (written as YAML):
	//
	// auths:
	//   (url):
	//     ... credentials ... here not relevant

	type aconfig map[string]interface{} // key: desired url
	var config map[string]aconfig       // key: "auths"

	err := json.Unmarshal(configjson, &config)
	if err != nil {
		return "", err
	}

	auths, ok := config["auths"]
	if !ok {
		return "", errors.New("missing key `auths` in `.dockerconfigjson`")
	}

	if len(auths) > 1 {
		return "", errors.New("more than one url found in `auths` in `.dockerconfigjson`")
	}
	if len(auths) < 1 {
		return "", errors.New("no url found in `auths` in `.dockerconfigjson`")
	}

	for key := range auths {
		// return the single url, as the first url found in the map
		return key + "/" + namespace, nil
	}

	// Cannot be reached
	return "", errors.New("registry.go / getRegistryUrlFromSecret - cannot happen")
}

func GetRegistryCredentialsFromSecret(secret v1.Secret) (RegistryCredentials, error) {
	empty := RegistryCredentials{}

	namespace, ok := secret.Annotations[RegistrySecretNamespaceAnnotationKey]
	if !ok {
		return empty, fmt.Errorf("missing annotation '%s'", RegistrySecretNamespaceAnnotationKey)
	}

	configjson, ok := secret.Data[".dockerconfigjson"]
	if !ok {
		return empty, errors.New("missing key `.dockerconfigjson`")
	}

	// The JSON has to have the following structure (written as YAML):
	//
	// auths:
	//   (url):
	//     ... credentials ... here not relevant

	type aconfig map[string]ContainerRegistryAuth // key, value: desired url, and creds
	var config map[string]aconfig                 // key: "auths"

	err := json.Unmarshal(configjson, &config)
	if err != nil {
		return empty, err
	}

	auths, ok := config["auths"]
	if !ok {
		return empty, errors.New("missing key `auths` in `.dockerconfigjson`")
	}

	if len(auths) > 1 {
		return empty, errors.New("more than one url found in `auths` in `.dockerconfigjson`")
	}
	if len(auths) < 1 {
		return empty, errors.New("no url found in `auths` in `.dockerconfigjson`")
	}

	for key, cred := range auths {
		username := cred.Username
		password := cred.Password

		// If username/password are empty but auth is set, decode the base64 auth field
		if (username == "" || password == "") && cred.Auth != "" {
			decoded, err := base64.StdEncoding.DecodeString(cred.Auth)
			if err == nil {
				decodedStr := string(decoded)
				parts := strings.SplitN(decodedStr, ":", 2)
				if len(parts) == 2 {
					username = parts[0]
					password = parts[1]
				}
			}
		}

		// return the single credentials, as the first credentials found in the map
		return RegistryCredentials{
			URL:      key + "/" + namespace,
			Username: username,
			Password: password,
		}, nil
	}

	// Cannot be reached
	return empty, errors.New("registry.go / getRegistryUrlFromSecret - cannot happen")
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
		return strings.ReplaceAll(imageURL, imageRegistryURL, privateURL), nil
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

	details.Namespace = secret.Annotations[RegistrySecretNamespaceAnnotationKey]

	for url, auth := range dockerconfigjson.Auths {
		username := auth.Username
		password := auth.Password

		// If username/password are empty but auth is set, decode the base64 auth field
		// This handles the standard dockerconfigjson format where credentials are stored
		// in the auth field as base64(username:password)
		if (username == "" || password == "") && auth.Auth != "" {
			decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
			if err == nil {
				decodedStr := string(decoded)
				parts := strings.SplitN(decodedStr, ":", 2)
				if len(parts) == 2 {
					username = parts[0]
					password = parts[1]
				}
			}
		}

		details.RegistryCredentials = append(details.RegistryCredentials, RegistryCredentials{
			URL:      url,
			Username: username,
			Password: password,
		})
	}

	return &details, nil
}

// DeleteImage deletes a container image from the registry using the Docker Registry HTTP API v2.
// It deletes all tags/manifests for the repository to ensure complete removal.
// It requires the image URL, registry credentials, and optionally a TLS config for self-signed certificates.
func DeleteImage(
	ctx context.Context,
	imageURL string,
	credentials RegistryCredentials,
	tlsConfig *tls.Config,
) error {
	if imageURL == "" {
		// No image to delete
		return nil
	}

	// Parse the image URL to extract registry, repository, and tag
	ref, err := parser.Parse(imageURL)
	if err != nil {
		return errors.Wrap(err, "parsing image URL")
	}

	registryURL := ref.Registry()
	repository := ref.ShortName()
	tag := ref.Tag()

	if tag == "" {
		tag = "latest"
	}

	helpers.Logger.Infow("Deleting image from registry", "image", imageURL, "repository", repository, "tag", tag)

	// Determine scheme from credentials URL (dockerconfigjson may contain http:// or https://)
	// or fall back to heuristics based on registry URL and TLS config
	scheme := "https"
	credURL := credentials.URL

	// Check if credentials URL has a scheme (it may include namespace suffix like "http://registry.com/namespace")
	if strings.HasPrefix(credURL, "http://") {
		scheme = "http"
	} else if strings.HasPrefix(credURL, "https://") {
		scheme = "https"
	} else {
		// No scheme in credentials URL, use heuristics
		// First check if registryURL already has a scheme
		if strings.HasPrefix(registryURL, "http://") {
			scheme = "http"
			registryURL = strings.TrimPrefix(registryURL, "http://")
		} else if strings.HasPrefix(registryURL, "https://") {
			scheme = "https"
			registryURL = strings.TrimPrefix(registryURL, "https://")
		} else if strings.HasPrefix(registryURL, "127.0.0.1") || strings.HasPrefix(registryURL, "localhost") || strings.HasPrefix(registryURL, "0.0.0.0") {
			// Localhost addresses are typically HTTP
			scheme = "http"
		}
		// Otherwise default to https (already set above)
		// Note: tlsConfig.InsecureSkipVerify is for skipping certificate verification on HTTPS,
		// not for switching to HTTP. Self-signed HTTPS registries still use https:// with
		// InsecureSkipVerify enabled.
	}

	// Get the manifest digest first
	manifestURL := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", scheme, registryURL, repository, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return errors.Wrap(err, "creating manifest request")
	}

	// Set authentication
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", credentials.Username, credentials.Password)))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json")

	// Create HTTP client with TLS config if provided
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if tlsConfig != nil {
		transport.TLSClientConfig = tlsConfig
	}
	client := &http.Client{
		Transport: transport,
	}
	resp, err := client.Do(req) // nolint:gosec // registry URL from cluster config, not user input
	if err != nil {
		return errors.Wrap(err, "fetching manifest")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		// Image doesn't exist, nothing to delete
		helpers.Logger.Infow("Image not found in registry, skipping deletion", "image", imageURL)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return errors.Errorf("failed to get manifest: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read the manifest body (we need it for computing digest if header is missing)
	manifestBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "reading manifest body")
	}

	// Get the digest from the response header (preferred)
	digest := resp.Header.Get("Docker-Content-Digest")

	// If digest is not in header, compute it from the manifest body
	if digest == "" {
		// Compute SHA256 digest of the manifest body
		hash := sha256.Sum256(manifestBody)
		digest = fmt.Sprintf("sha256:%s", hex.EncodeToString(hash[:]))
		helpers.Logger.Infow("Computed digest from manifest body", "digest", digest, "image", imageURL)
	}

	// First, list all tags for this repository so we can delete them all
	// This ensures complete removal of the repository from the catalog
	allTags, err := listRepositoryTags(ctx, scheme, registryURL, repository, auth, client)
	if err != nil {
		helpers.Logger.Infow("Could not list tags for repository, will delete only the specified tag",
			"repository", repository,
			"error", err)
		allTags = []string{tag} // Fall back to just deleting the specified tag
	} else if len(allTags) == 0 {
		helpers.Logger.Infow("Repository has no tags, nothing to delete", "repository", repository)
		return nil
	} else {
		helpers.Logger.Infow("Found tags for repository, will delete all of them",
			"repository", repository,
			"tagCount", len(allTags),
			"tags", allTags)
	}

	// Delete all tags for this repository
	var lastErr error
	for _, tagToDelete := range allTags {
		if err := deleteTagByTag(ctx, scheme, registryURL, repository, tagToDelete, auth, client); err != nil {
			// Log error but continue with other tags
			helpers.Logger.Errorw("Failed to delete tag", "repository", repository, "tag", tagToDelete, "error", err)
			lastErr = err
		}
	}

	if lastErr != nil {
		return errors.Wrap(lastErr, "some tags failed to delete")
	}

	helpers.Logger.Infow("Successfully deleted all tags from repository",
		"repository", repository,
		"tagCount", len(allTags))
	return nil
}

// listRepositoryTags lists all tags for a repository
func listRepositoryTags(ctx context.Context, scheme, registryURL, repository, auth string, client *http.Client) ([]string, error) {
	tagsListURL := fmt.Sprintf("%s://%s/v2/%s/tags/list", scheme, registryURL, repository)

	listReq, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsListURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating tags list request")
	}

	listReq.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))

	listResp, err := client.Do(listReq) // nolint:gosec // registry URL from cluster config, not user input
	if err != nil {
		return nil, errors.Wrap(err, "listing tags")
	}
	defer func() {
		_ = listResp.Body.Close()
	}()

	if listResp.StatusCode == http.StatusNotFound {
		// Repository doesn't exist or has no tags
		return []string{}, nil
	}

	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		return nil, errors.Errorf("failed to list tags: status %d, body: %s", listResp.StatusCode, string(body))
	}

	// Parse the tags list response
	var tagsList struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}

	if err := json.NewDecoder(listResp.Body).Decode(&tagsList); err != nil {
		return nil, errors.Wrap(err, "parsing tags list response")
	}

	return tagsList.Tags, nil
}

// deleteTagByTag deletes a tag by fetching its manifest and deleting by digest
func deleteTagByTag(
	ctx context.Context,
	scheme,
	registryURL,
	repository,
	tag,
	auth string,
	client *http.Client,
) error {
	// Get the manifest for this tag to get its digest
	manifestURL := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", scheme, registryURL, repository, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return errors.Wrap(err, "creating manifest request")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json")

	resp, err := client.Do(req) // nolint:gosec // registry URL from cluster config, not user input
	if err != nil {
		return errors.Wrap(err, "fetching manifest")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		// Tag doesn't exist, skip it
		helpers.Logger.Infow("Tag not found, skipping", "repository", repository, "tag", tag)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return errors.Errorf("failed to get manifest: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Read manifest body to compute digest
	manifestBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "reading manifest body")
	}

	// Get digest from header or compute it
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		hash := sha256.Sum256(manifestBody)
		digest = fmt.Sprintf("sha256:%s", hex.EncodeToString(hash[:]))
	}

	// Delete by digest (required by Docker Registry API v2)
	deleteURL := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", scheme, registryURL, repository, digest)
	deleteReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		return errors.Wrap(err, "creating delete request")
	}

	deleteReq.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))
	acceptHeader := resp.Header.Get("Content-Type")
	if acceptHeader == "" {
		acceptHeader = "application/vnd.docker.distribution.manifest.v2+json"
	}
	deleteReq.Header.Set("Accept", acceptHeader)

	deleteResp, err := client.Do(deleteReq) // nolint:gosec // registry URL from cluster config, not user input
	if err != nil {
		return errors.Wrap(err, "deleting manifest")
	}
	defer func() {
		_ = deleteResp.Body.Close()
	}()

	if deleteResp.StatusCode == http.StatusAccepted ||
		deleteResp.StatusCode == http.StatusOK ||
		deleteResp.StatusCode == http.StatusNotFound {
		helpers.Logger.Infow("Deleted tag from repository", "repository", repository, "tag", tag, "digest", digest)
		return nil
	}

	// Check if deletion is disabled
	if deleteResp.StatusCode == http.StatusMethodNotAllowed {
		helpers.Logger.Infow("Image deletion is disabled on this registry for this tag", "repository", repository, "tag", tag)
		return nil // Don't fail - registry doesn't support deletion
	}

	body, _ := io.ReadAll(deleteResp.Body)
	return errors.Errorf("failed to delete tag: status %d, body: %s", deleteResp.StatusCode, string(body))
}
