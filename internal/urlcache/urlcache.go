// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/epinio/epinio/helpers"
	"github.com/pkg/errors"
)

var BasePath = "/tmp/urlcache"

// syncURLMap is holding a mutex for each url
var syncURLMap sync.Map

// initMutex serializes the cache directory initialization
var initMutex sync.Mutex

// Get returns the local file containing the data found at the specified url.
// It fetches the url when the local file does not exist yet.
func Get(ctx context.Context, url string) (string, error) {
	helpers.Logger.Debugw("get", "url", url)

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
		helpers.Logger.Debugw("is local file")
		return url, nil
	}

	// Compute cache path for url
	h := sha256.New()
	h.Write([]byte(url))
	hash := h.Sum(nil)
	fileName := base64.RawURLEncoding.EncodeToString([]byte(hash))
	path := filepath.Join(BasePath, fileName+".tgz")

	// Check cache for url, and return if already cached
	if _, err := os.Stat(path); err == nil {
		helpers.Logger.Debugw("cache HIT", "path", path)
		return path, nil
	}

	// Initialize cache
	err := initCache()
	if err != nil {
		return "", errors.Wrap(err, "unable to setup url cache")
	}

	// Extend cache
	err = fetchFile(url, path)
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch url")
	}

	// Return cache element
	return path, nil
}

func initCache() error {
	// DANGER: The initialization of the cache directory requires serialization as well, so that
	// DANGER: only access performs the initialization, while everything else sees the
	// DANGER: initialized directory. This is url-independent and not ensures by the main map,
	// DANGER: which serializes per url.

	initMutex.Lock()
	defer initMutex.Unlock()

	if _, err := os.Stat(BasePath); err != nil {
		helpers.Logger.Debugw("initialize cache", "directory", BasePath)
		// Not yet initialized
		err := os.MkdirAll(BasePath, 0700)
		if err != nil {
			return err
		}
	}

	return nil
}

func fetchFile(originURL, destinationPath string) error {
	helpers.Logger.Debugw(
		"cache MISS, fetch",
		"url",
		originURL,
		"path",
		destinationPath,
	)

	response, err := http.Get(originURL) // nolint:gosec // app chart repo ref
	if err != nil {
		return err
	}
	if response.StatusCode >= http.StatusBadRequest {
		helpers.Logger.Debugw("fail http", "status", response.StatusCode)
		return fmt.Errorf("failed with status %d", response.StatusCode)
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			helpers.Logger.Errorw("failed to close response body", "error", err)
		}
	}()

	dstFile, err := os.Create(destinationPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(dstFile, response.Body)
	if err != nil {
		dstFileError := dstFile.Close()
		if dstFileError != nil {
			return dstFileError
		}

		fileRemoveError := os.Remove(dstFile.Name()) // nolint:gosec // path from os.Create in same flow
		if fileRemoveError != nil {
			return fileRemoveError
		}
		return err
	}

	dstFileError := dstFile.Close()
	if dstFileError != nil {
		return dstFileError
	}

	// Verify fetch (path has to exist now)
	stat, err := os.Stat(destinationPath)
	if err != nil {
		return err
	}

	helpers.Logger.Debugw(
		"now cached",
		"path",
		destinationPath,
		"size",
		stat.Size(),
	)
	return nil
}
