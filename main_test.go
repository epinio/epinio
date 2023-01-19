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

package main

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"
)

// Only run our coverage binary when EPINIO_COVERAGE is set, do not run for
// normal unit tests.
func TestSystem(_ *testing.T) {
	if _, ok := os.LookupEnv("EPINIO_COVERAGE"); ok {
		if _, ok := os.LookupEnv("KUBERNETES_SERVICE_HOST"); ok {
			flag.Set("test.coverprofile", "/tmp/coverprofile.out")
		} else {
			// running as CLI, don't overwrite existing files
			flag.Set("test.coverprofile", fmt.Sprintf("/tmp/coverprofile%d.out", time.Now().Unix()))
		}
		main()
	}
}
