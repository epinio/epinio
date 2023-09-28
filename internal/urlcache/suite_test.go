// Copyright © 2023-2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package urlcache_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/epinio/epinio/internal/urlcache"
	"github.com/go-logr/logr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEpinio(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "URL Cache Suite")
}

var logger = logr.Discard()

var _ = BeforeSuite(func() {
	fmt.Printf("Running tests on node %d\n", GinkgoParallelProcess())

	// Redirect the url cache to a local temp directory
	// This is done as suite-wide initialization to prevent races between
	// concurrent tests initializating and deleting the test-local url cache.
	// Also ensured that different nodes use different caches.
	// Else we have races between the nodes

	here, _ := os.Getwd()
	dir := fmt.Sprintf("uc%d", GinkgoParallelProcess())
	urlcache.BasePath = filepath.Join(here, dir)

	fmt.Printf("Caching at %s\n", urlcache.BasePath)
})

var _ = AfterSuite(func() {
	// And remove the local url cache directory again
	fmt.Printf("Dropping cache %s\n", urlcache.BasePath)

	_ = os.RemoveAll(urlcache.BasePath)
})
