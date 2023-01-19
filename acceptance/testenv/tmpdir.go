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

package testenv

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/epinio/epinio/acceptance/helpers/proc"
)

const (
	// skipCleanupPath is the path (relative to the test
	// directory) of a file which, when present causes the system
	// to not delete the test cluster after the tests are done.
	skipCleanupPath = "/tmp/skip_cleanup"
)

// SkipCleanup returns true if the file exists, false if some error occurred
// while checking
func SkipCleanup() bool {
	_, err := os.Stat(root + skipCleanupPath)
	return err == nil
}

func SkipCleanupPath() string {
	return root + skipCleanupPath
}

func DeleteTmpDir(nodeTmpDir string) {
	err := os.RemoveAll(nodeTmpDir)
	if err != nil {
		panic(fmt.Sprintf("Failed deleting temp dir %s: %s\n",
			nodeTmpDir, err.Error()))
	}
}

// Remove all tmp directories from /tmp/epinio-* . Test should try to cleanup
// after themselves but that sometimes doesn't happen, either because we forgot
// the cleanup code or because the test failed before that happened.
// NOTE: This code will create problems if more than one acceptance_suite_test.go
// is run in parallel (e.g. two PRs on one worker). However we keep it as an
// extra measure.
func CleanupTmp() (string, error) {
	temps, err := filepath.Glob("/tmp/epinio-*")
	if err != nil {
		return "", err
	}
	return proc.Run("", true, "rm", append([]string{"-rf"}, temps...)...)
}

// CopyEpinioSettings copies the epinio yaml to the given dir
func CopyEpinioSettings(dir string) (string, error) {
	return proc.Run("", false, "cp", EpinioYAML(), dir+"/epinio.yaml")
}
