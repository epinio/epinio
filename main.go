package main

import (
	"log"
	"os"
	"runtime/pprof"

	"github.com/epinio/epinio/internal/cli"
)

func main() {
	f, err := os.Create("cpuprofile")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	cli.Execute()
}
