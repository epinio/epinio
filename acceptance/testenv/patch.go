package testenv

import (
	"fmt"
	"os"

	"github.com/epinio/epinio/acceptance/helpers/proc"
)

func PatchEpinio() (string, error) {
	if os.Getenv(skipEpinioPatch) != "" {
		return "", nil
	}
	// Patch Epinio deployment to inject the current binary
	fmt.Println("Patching Epinio deployment with test binary")
	return proc.Run(Root(), false, "make", "patch-epinio-deployment")
}
