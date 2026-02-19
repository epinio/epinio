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

// Package registry: check if a container image exists in a registry (Docker Hub, OCI).
// The buildpacks registry-index (https://github.com/buildpacks/registry-index) indexes
// buildpacks, not builder images; builder images live in container registries, so we
// use the Registry API v2 to verify existence.

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	parser "github.com/novln/docker-parser"
)

const (
	manifestCheckTimeout = 10 * time.Second
	acceptManifest       = "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json"
)

var (
	dockerHubAuthURL  = "https://auth.docker.io/token"
	dockerHubRegistry = "registry-1.docker.io"
)

// dockerTokenResponse is the response from auth.docker.io/token
type dockerTokenResponse struct {
	Token string `json:"token"`
}

// ImageExistsInRegistry checks whether the given image reference exists in its
// container registry (Docker Hub, GHCR, or other OCI v2 registries). It does
// not use the buildpacks registry-index, which indexes buildpacks, not builder
// images. Returns true if the image exists (200), false if not found (404), and
// an error on other failures (e.g. timeout, 5xx).
func ImageExistsInRegistry(ctx context.Context, imageRef string) (exists bool, err error) {
	ref, err := parser.Parse(imageRef)
	if err != nil {
		return false, err
	}
	registryAPIHost := normalizeRegistryAPIHost(ref.Registry())
	repository := ref.ShortName()
	tag := ref.Tag()
	if tag == "" {
		tag = "latest"
	}
	baseURL := "https://" + registryAPIHost
	manifestURL := fmt.Sprintf("%s/v2/%s/manifests/%s", baseURL, repository, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", acceptManifest)

	// Docker Hub requires a Bearer token (even for public pulls)
	if registryAPIHost == dockerHubRegistry {
		token, tokenErr := getDockerHubToken(ctx, repository)
		if tokenErr != nil {
			return false, tokenErr
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: manifestCheckTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("registry returned %d for %s", resp.StatusCode, imageRef)
	}
}

// RepositoryExistsInRegistry checks whether a repository exists in a registry.
// This avoids false negatives caused by checking only a specific tag (e.g. latest).
func RepositoryExistsInRegistry(ctx context.Context, registryHost, repository string) (bool, error) {
	registryAPIHost := normalizeRegistryAPIHost(registryHost)
	baseURL := "https://" + registryAPIHost
	tagsURL := fmt.Sprintf("%s/v2/%s/tags/list?n=1", baseURL, repository)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, nil)
	if err != nil {
		return false, err
	}
	if registryAPIHost == dockerHubRegistry {
		token, tokenErr := getDockerHubToken(ctx, repository)
		if tokenErr != nil {
			return false, tokenErr
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: manifestCheckTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("registry returned %d for repository %s", resp.StatusCode, repository)
	}
}

func getDockerHubToken(ctx context.Context, repository string) (string, error) {
	scope := "repository:" + repository + ":pull"
	authURL := dockerHubAuthURL + "?service=registry.docker.io&scope=" + url.QueryEscape(scope)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth.docker.io returned %d", resp.StatusCode)
	}
	var out dockerTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", fmt.Errorf("no token in auth response")
	}
	return out.Token, nil
}

func normalizeRegistryAPIHost(registryHost string) string {
	if registryHost == "" || registryHost == "docker.io" || registryHost == "index.docker.io" {
		return dockerHubRegistry
	}
	registryAPIHost := strings.TrimPrefix(registryHost, "https://")
	registryAPIHost = strings.TrimPrefix(registryAPIHost, "http://")
	return registryAPIHost
}
