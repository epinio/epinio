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

	"github.com/epinio/epinio/acceptance/helpers/proc"
	. "github.com/onsi/ginkgo/v2"
)

// RegistryUsername returns the docker registry username from the env
func RegistryUsername() string {
	return os.Getenv(registryUsernameEnv)
}

// RegistryPassword returns the docker registry password from the env
func RegistryPassword() string {
	return os.Getenv(registryPasswordEnv)
}

// CreateRegistrySecret creates the docker registry image pull secret
func CreateRegistrySecret() {
	if RegistryUsername() != "" && RegistryPassword() != "" {
		fmt.Printf("Creating image pull secret for Dockerhub on node %d\n", GinkgoParallelProcess())
		_, _ = proc.Kubectl("create", "secret", "docker-registry", "regcred",
			"--docker-server", "https://index.docker.io/v1/",
			"--docker-username", RegistryUsername(),
			"--docker-password", RegistryPassword(),
		)
	}
}
