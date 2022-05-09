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
