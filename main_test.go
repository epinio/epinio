package main

import (
	"flag"
	"os"
	"testing"
)

// Only run our coverage binary when EPINIO_COVERAGE is set, do not run for
// normal unit tests.
func TestSystem(_ *testing.T) {
	if _, ok := os.LookupEnv("EPINIO_COVERAGE"); ok {
		flag.Set("test.coverprofile", "/tmp/coverprofile.out")
		main()
	}
}
