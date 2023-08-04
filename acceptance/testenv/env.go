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
)

func SetupEnv() {
	if RegistryUsername() == "" || RegistryPassword() == "" {
		fmt.Println("REGISTRY_USERNAME or REGISTRY_PASSWORD environment variables are empty. Pulling from dockerhub will be subject to rate limiting.")
	}

	// this env var is for the patch-epinio-deployment target in the
	// Makefile, which has the top level as root dir
	if os.Getenv("EPINIO_BINARY_PATH") == "" {
		os.Setenv("EPINIO_BINARY_PATH", fmt.Sprintf("./dist/%s", ServerBinaryName()))
	}
}
