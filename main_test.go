package main

import (
	"log"
	"testing"

	"github.com/epinio/epinio/internal/cli"
)

func TestMain(t *testing.T) {
	log.Println(cli.RunServer())
}
