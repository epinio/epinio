package testenv

import (
	"fmt"
	"os"

	"github.com/epinio/epinio/helpers"
	"github.com/onsi/ginkgo/config"
)

func RegistryUsername() string {
	return os.Getenv(registryUsernameEnv)
}

func RegistryPassword() string {
	return os.Getenv(registryPasswordEnv)
}

func CreateRegistrySecret() {
	if RegistryUsername() != "" && RegistryPassword() != "" {
		fmt.Printf("Creating image pull secret for Dockerhub on node %d\n", config.GinkgoConfig.ParallelNode)
		_, _ = helpers.Kubectl(fmt.Sprintf("create secret docker-registry regcred --docker-server=%s --docker-username=%s --docker-password=%s",
			"https://index.docker.io/v1/",
			RegistryUsername(),
			RegistryPassword(),
		))
	}
}
