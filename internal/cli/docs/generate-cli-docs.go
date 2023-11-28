// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/epinio/epinio/internal/cli"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func main() {
	docDir := filepath.Join("./", os.Args[1])

	cmd, err := cli.NewRootCmd()
	if err != nil {
		panic(err)
	}

	cmd.DisableAutoGenTag = true

	err = generateCmdDoc(cmd, docDir)
	if err != nil {
		panic(err)
	}
}

// generateCmdDoc will generate the documentation for the given command, in the given directory
func generateCmdDoc(cmd *cobra.Command, dir string) error {
	if cmd.Hidden {
		return nil
	}

	// create the directory if it doesn't exist
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return errors.Wrapf(err, "error creating directory [%s]", dir)
	}

	// create the _category_.json file
	err = createCategoryJSONFile(cmd.Name(), dir)
	if err != nil {
		return errors.Wrapf(err, "error creating _category_.json file for command [%s]", cmd.Name())
	}

	// create the documentation for the given command
	err = createMarkdownFile(cmd, dir)
	if err != nil {
		return errors.Wrapf(err, "error creating markdown file for command [%s]", cmd.Name())
	}

	// create the documentation for its subcommands
	for _, subcmd := range cmd.Commands() {
		// skip the 'epinio push' in the 'app' folder
		if subcmd.CommandPath() == "epinio push" && filepath.Base(dir) == "app" {
			continue
		}

		// if the subcommand does not have other subcommands, just generate the doc and continue
		if !subcmd.HasSubCommands() {
			err = createMarkdownFile(subcmd, dir)
			if err != nil {
				return errors.Wrapf(err, "error creating markdown file for command [%s]", subcmd.Name())
			}
			continue
		}

		// if the subcommand has other subcommands, then recurse in its own directory
		subdir := filepath.Join(dir, subcmd.Name())
		err = generateCmdDoc(subcmd, subdir)
		if err != nil {
			return errors.Wrapf(err, "error generating doc for command [%s]", subcmd.Name())
		}
	}

	return nil
}

// createMarkdownFile will create the markdown file for the given command in the given directory
func createMarkdownFile(cmd *cobra.Command, dir string) error {
	// skip 'help' command
	if cmd.Name() == "help" {
		return nil
	}

	basename := strings.Replace(cmd.CommandPath(), " ", "_", -1) + ".md"
	filename := filepath.Join(dir, basename)

	f, err := os.Create(filename)
	if err != nil {
		return errors.Wrap(err, "error creating file")
	}
	defer f.Close()

	err = writeFileHeader(f, cmd.CommandPath())
	if err != nil {
		return errors.Wrapf(err, "error writing file header for command [%s]", cmd.Name())
	}

	err = doc.GenMarkdownCustom(cmd, f, linkHandler(cmd, dir))
	return errors.Wrap(err, "error generating markdown custom")
}

// createCategoryJSONFile creates the '_category_.json' in the given directory
func createCategoryJSONFile(label, dir string) error {
	f, err := os.Create(filepath.Join(dir, "_category_.json"))
	if err != nil {
		return errors.Wrap(err, "error creating file")
	}
	defer f.Close()

	cat := struct {
		Label     string `json:"label"`
		Collapsed bool   `json:"collapsed"`
	}{
		Label:     label,
		Collapsed: true,
	}

	b, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return errors.Wrap(err, "error encoding json")
	}

	_, err = fmt.Fprintln(f, string(b))
	return errors.Wrap(err, "error writing json to file")
}

// linkHandler will return a function that will handle the markdown link generation
func linkHandler(cmd *cobra.Command, _ string) func(link string) string {
	return func(link string) string {
		cmdPathLink := strings.Replace(strings.TrimSuffix(link, ".md"), "_", " ", -1)

		// check if the link was referring to the parent command
		// we also need to check if the current command has subcommands, because if it does not then the link needs to point to the same directory
		if cmd.HasParent() && cmd.Parent().CommandPath() == cmdPathLink && cmd.HasSubCommands() {
			return "../" + link
		}

		for _, subcmd := range cmd.Commands() {
			// if the subcommand has other subcommands then it will have its own directory
			if subcmd.CommandPath() == cmdPathLink && subcmd.HasSubCommands() {
				return fmt.Sprintf("./%s/%s", subcmd.Name(), link)
			}
		}

		// fix for alias
		if link == "epinio_app_push.md" {
			return "../epinio_push.md"
		}

		return "./" + link
	}
}

func writeFileHeader(w io.Writer, sidebarLabel string) error {
	if sidebarLabel == "epinio" {
		sidebarLabel = "epinio cli"
	}
	description := sidebarLabel
	keywords := sidebarLabel
	docTopic := strings.ReplaceAll(sidebarLabel, " ", "-")

	_, err := fmt.Fprintf(w, `---
sidebar_label: %s
title: ""
description: %s
keywords: [epinio, kubernetes, %s]
doc-type: [reference]
doc-topic: [epinio, reference, epinio-cli, %s]
doc-persona: [epinio-developer, epinio-operator]
---
`, sidebarLabel, description, keywords, docTopic)

	return errors.Wrap(err, "error writing file header")
}
