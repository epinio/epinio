package testenv

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
)

func SetupEnv() {
	if RegistryUsername() == "" || RegistryPassword() == "" {
		fmt.Println("REGISTRY_USERNAME or REGISTRY_PASSWORD environment variables are empty. Pulling from dockerhub will be subject to rate limiting.")
	}

	// this env var is for the patch-epinio-deployment target in the
	// Makefile, which has the top level as root dir
	if os.Getenv("EPINIO_BINARY_PATH") == "" {
		serverBinary := fmt.Sprintf("%s/dist/%s", Root(), ServerBinaryName())
		By("Server Binary (Sys): " + serverBinary)
		os.Setenv("EPINIO_BINARY_PATH", serverBinary)
	} else {
		By("Server Binary (Env): " + os.Getenv("EPINIO_BINARY_PATH"))
	}
	os.Setenv("EPINIO_DONT_WAIT_FOR_DEPLOYMENT", "1")
	os.Setenv("SKIP_SSL_VERIFICATION", "true")
}
