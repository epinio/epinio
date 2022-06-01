package main

import (
	"os"
	"path/filepath"

	"github.com/epinio/epinio/internal/cli"
	"github.com/spf13/cobra/doc"
)

func main() {
	docDir := filepath.Join("./", os.Args[1])

	cmd := cli.NewEpinioCLI()
	cmd.DisableAutoGenTag = true

	err := doc.GenMarkdownTreeCustom(cmd, docDir, filePrepender(docDir), linkHandler)
	if err != nil {
		panic(err)
	}
}

func filePrepender(docDir string) func(string) string {
	return func(file string) string {
		return "---\ntitle: \"\"\n---\n"
	}
}

func linkHandler(link string) string {
	if link == "epinio_app_push.md" {
		link = "epinio_push.md"
	}
	return "./" + link
}
