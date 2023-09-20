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

// Package urlcache maintains a local cache of files for external urls.
package urlcache

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

type Fetcher func(ctx context.Context, logger logr.Logger, originURL, destinationPath string) error

const BasePath = "/tmp/urlcache"

// syncURLMap is holding a mutex for each url
var syncURLMap sync.Map

// Get returns the local file containing the data found at the specified url.
// It invokes the fetcher when the local file does not exist yet.
func Get(ctx context.Context, logger logr.Logger, url string, fetcher Fetcher) (string, error) {
	logger = logger.V(1).WithName("URLCache")

	logger.Info("get", "url", url)

	// DANGER: The url cache is a global structure shared among all API requests.
	// DANGER: It is very possible that multiple goroutines invoke `Get` for the
	// DANGER: same url, at nearly the same time.
	//
	// Here we perform per-url interlocking so that only one of these goroutines
	// will perform the fetch, while all others are blocked until the fetcher is done.

	anyMutex, _ := syncURLMap.LoadOrStore(url, &sync.Mutex{})
	if m, ok := anyMutex.(*sync.Mutex); ok {
		m.Lock()
		defer m.Unlock()
	}

	// No caching needed for local file.

	if _, err := os.Stat(url); err == nil {
		logger.Info("is local file")
		return url, nil
	}

	// Compute cache path for url

	h := sha256.New()
	h.Write([]byte(url))
	hash := h.Sum(nil)
	fileName := strings.ReplaceAll(base64.StdEncoding.EncodeToString([]byte(hash)), "/", ".")
	path := filepath.Join(BasePath, fileName+".tgz")

	// Check cache for url

	if _, err := os.Stat(path); err == nil {
		logger.Info("is cached", "path", path)
		// Already cached
		return path, nil
	}

	// Initialize cache

	if _, err := os.Stat(BasePath); err != nil {
		logger.Info("initialize cache", "directory", BasePath)
		// Not yet initialized
		err := os.MkdirAll(BasePath, 0700)
		if err != nil {
			return "", errors.Wrapf(err, "unable to setup url cache")
		}
	}

	// Extend cache
	logger.Info("fetch", "url", url, "path", path)

	err := fetcher(ctx, logger, url, path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to fetch url")
	}

	// Verify fetch (path has to exist now)

	stat, err := os.Stat(path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to properly save fetched url")
	}

	logger.Info("now cached", "path", path, "size", stat.Size())
	return path, nil
}

func HttpFetcher(ctx context.Context, logger logr.Logger, originURL, destinationPath string) error {
	response, err := http.Get(originURL) // nolint:gosec // app chart repo ref
	if err != nil || response.StatusCode != http.StatusOK {
		logger.Info("fail, http issue")
		return err
	}
	defer response.Body.Close()

	dstFile, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, response.Body)
	return err

}
