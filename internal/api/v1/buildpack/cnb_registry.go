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

package buildpack

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

const (
	registryIndexRepo   = "buildpacks/registry-index"
	registryIndexBranch = "main"
	githubTreeURL       = "https://api.github.com/repos/%s/git/trees/%s?recursive=1"
	rawContentURL       = "https://raw.githubusercontent.com/%s/%s/%s"
	maxBlobsToFetch     = 25
	maxBuildpackEntries = 50
	treeCacheTTL        = 5 * time.Minute
	blobCacheTTL        = 5 * time.Minute
)

var cnbRegistryHTTPClient = &http.Client{Timeout: 8 * time.Second}

type githubTree struct {
	Tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"tree"`
}

type ndjsonLine struct {
	Ns      string `json:"ns"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Yanked  bool   `json:"yanked"`
}

type treeCacheState struct {
	expiresAt time.Time
	tree      githubTree
}

type blobCacheState struct {
	expiresAt time.Time
	lines     []ndjsonLine
}

var (
	cacheMu           sync.RWMutex
	cachedTree        treeCacheState
	cachedBlobContent = map[string]blobCacheState{}
)

// SearchCNBRegistry searches the CNB registry index (GitHub buildpacks/registry-index)
// by term and returns up to maxBuildpackEntries buildpack entries with versions and latest.
func SearchCNBRegistry(ctx context.Context, searchTerm string) (*models.BuildpackSearchResponse, error) {
	term := strings.TrimSpace(strings.ToLower(searchTerm))
	if term == "" {
		return &models.BuildpackSearchResponse{Buildpacks: []models.BuildpackEntry{}}, nil
	}

	tree, err := loadRegistryTree(ctx)
	if err != nil {
		return nil, err
	}

	// Collect blob paths that match the search term (path is like "2/paketo-buildpacks_go" or "3/jv/heroku_jvm")
	var matchingPaths []string
	for _, n := range tree.Tree {
		if n.Type != "blob" {
			continue
		}
		pathLower := strings.ToLower(n.Path)
		if strings.Contains(pathLower, term) {
			matchingPaths = append(matchingPaths, n.Path)
		}
	}
	if len(matchingPaths) > maxBlobsToFetch {
		matchingPaths = matchingPaths[:maxBlobsToFetch]
	}

	// Aggregate by id (ns/name); collect versions and latest
	byID := make(map[string]*models.BuildpackEntry)

	for _, path := range matchingPaths {
		entries, err := fetchAndParseNDJSON(ctx, path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			id := e.Ns + "/" + e.Name
			if e.Yanked {
				continue
			}
			if _, ok := byID[id]; !ok {
				byID[id] = &models.BuildpackEntry{ID: id, Versions: []string{}}
			}
			entry := byID[id]
			entry.Versions = append(entry.Versions, e.Version)
		}
	}

	// Dedupe versions and set latest (max by string compare)
	result := make([]models.BuildpackEntry, 0, len(byID))
	for _, entry := range byID {
		versions := uniqueSortedVersions(entry.Versions)
		latest := ""
		if len(versions) > 0 {
			latest = versions[len(versions)-1]
		}
		entry.Versions = versions
		entry.Latest = latest
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	if len(result) > maxBuildpackEntries {
		result = result[:maxBuildpackEntries]
	}
	return &models.BuildpackSearchResponse{Buildpacks: result}, nil
}

func loadRegistryTree(ctx context.Context) (githubTree, error) {
	cacheMu.RLock()
	if time.Now().Before(cachedTree.expiresAt) {
		tree := cachedTree.tree
		cacheMu.RUnlock()
		return tree, nil
	}
	cacheMu.RUnlock()

	treeURL := fmt.Sprintf(githubTreeURL, registryIndexRepo, registryIndexBranch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, treeURL, nil)
	if err != nil {
		return githubTree{}, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := cnbRegistryHTTPClient.Do(req)
	if err != nil {
		return githubTree{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return githubTree{}, fmt.Errorf("github tree API returned %d", resp.StatusCode)
	}

	var tree githubTree
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return githubTree{}, err
	}

	cacheMu.Lock()
	cachedTree = treeCacheState{
		tree:      tree,
		expiresAt: time.Now().Add(treeCacheTTL),
	}
	cacheMu.Unlock()

	return tree, nil
}

func fetchAndParseNDJSON(ctx context.Context, path string) ([]ndjsonLine, error) {
	cacheMu.RLock()
	if cachedBlob, ok := cachedBlobContent[path]; ok && time.Now().Before(cachedBlob.expiresAt) {
		lines := cachedBlob.lines
		cacheMu.RUnlock()
		return lines, nil
	}
	cacheMu.RUnlock()

	rawURL := fmt.Sprintf(rawContentURL, registryIndexRepo, registryIndexBranch, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := cnbRegistryHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("raw content returned %d", resp.StatusCode)
	}

	var lines []ndjsonLine
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var l ndjsonLine
		if json.Unmarshal([]byte(line), &l) != nil {
			continue
		}
		lines = append(lines, l)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	cacheMu.Lock()
	cachedBlobContent[path] = blobCacheState{
		lines:     lines,
		expiresAt: time.Now().Add(blobCacheTTL),
	}
	cacheMu.Unlock()

	return lines, nil
}

func uniqueSortedVersions(versions []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, v := range versions {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return versionLess(out[i], out[j]) })
	return out
}

func versionLess(a, b string) bool {
	aSemver, errA := semver.NewVersion(strings.TrimPrefix(a, "v"))
	bSemver, errB := semver.NewVersion(strings.TrimPrefix(b, "v"))
	if errA == nil && errB == nil {
		return aSemver.LessThan(bSemver)
	}
	if errA == nil && errB != nil {
		return false
	}
	if errA != nil && errB == nil {
		return true
	}
	return a < b
}
