// Copyright © 2021 - 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package catalog

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// DownloadChartArchive downloads a helm chart .tgz from url into a temp file and
// returns its path. Used by app-chart acceptance tests.
func DownloadChartArchive(url string) string {
	GinkgoHelper()

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url) // nolint:gosec // fixed release URL from tests
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = resp.Body.Close() }()
	Expect(resp.StatusCode).To(Equal(http.StatusOK), fmt.Sprintf("download %s: %s", url, resp.Status))

	path := filepath.Join(GinkgoT().TempDir(), "epinio-application.tgz")
	file, err := os.Create(path) // nolint:gosec // temp path under test control
	Expect(err).NotTo(HaveOccurred())

	_, err = io.Copy(file, resp.Body)
	closeErr := file.Close()
	Expect(err).NotTo(HaveOccurred())
	Expect(closeErr).NotTo(HaveOccurred())

	info, err := os.Stat(path)
	Expect(err).NotTo(HaveOccurred())
	Expect(info.Size()).To(BeNumerically(">", 100), "chart archive too small")

	return path
}
