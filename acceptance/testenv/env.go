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
	os.Setenv("SKIP_SSL_VERIFICATION", "true")
}
