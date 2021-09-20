package testenv

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/proc"
)

func PatchEpinio() (string, error) {
	// Patch Epinio deployment to inject the current binary
	fmt.Println("Patching Epinio deployment with test binary")
	return proc.Run(Root(), false, "bash", "./scripts/patch-epinio-deployment.sh")
}
