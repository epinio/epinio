package testenv

import (
	"fmt"
	"os"
	"path"
)

func SetupEnv() {
	if RegistryUsername() == "" || RegistryPassword() == "" {
		fmt.Println("REGISTRY_USERNAME or REGISTRY_PASSWORD environment variables are empty. Pulling from dockerhub will be subject to rate limiting.")
	}

	// this env var is for the Makefile, which has the top level as root dir
	os.Setenv("EPINIO_BINARY_PATH", path.Join("dist", "epinio-linux-amd64"))
	os.Setenv("EPINIO_DONT_WAIT_FOR_DEPLOYMENT", "1")
	os.Setenv("EPINIO_CONFIG", EpinioYAML())
	os.Setenv("SKIP_SSL_VERIFICATION", "true")

}
