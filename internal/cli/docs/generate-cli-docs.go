package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/epinio/epinio/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var markdownURLsToDocsy = func(s string) string {
	s = strings.ReplaceAll(s, ".md", "")
	s = "../" + s
	return s
}

func docsyPrepend(s, docDir string) string {
	title := strings.ReplaceAll(s, docDir, "")
	title = strings.ReplaceAll(title, ".md", "")
	title = strings.ReplaceAll(title, "_", " ")

	return `---
title: "` + title + `"
linkTitle: "` + title + `"
weight: 1
---
`
}

func genCLIDocsyMarkDown(cmd *cobra.Command, docDir string) error {
	return doc.GenMarkdownTreeCustom(cmd, filepath.Join("./", docDir), func(s string) string { return docsyPrepend(s, docDir) }, markdownURLsToDocsy)
}

func main() {
	docDir := os.Args[1]
	if err := genCLIDocsyMarkDown(cli.NewEpinioCLI(), docDir); err != nil {
		panic(err)
	}
}
