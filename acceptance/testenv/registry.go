package testenv

import (
	"fmt"
	"os"

	"github.com/epinio/epinio/helpers"
	"github.com/onsi/ginkgo/config"
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
		fmt.Printf("Creating image pull secret for Dockerhub on node %d\n", config.GinkgoConfig.ParallelNode)
		_, _ = helpers.Kubectl("create", "secret", "docker-registry", "regcred",
			"--docker-server", "https://index.docker.io/v1/",
			"--docker-username", RegistryUsername(),
			"--docker-password", RegistryPassword(),
		)
	}
}
