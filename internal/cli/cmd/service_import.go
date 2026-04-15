// Copyright © 2026 - SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0

package cmd

import (
	"github.com/spf13/cobra"
)

// NewServiceImportCmd wires `epinio service import`.
//
// Usage:
//
//	epinio service import \
//	  --source-namespace data \
//	  --release postgres-db \
//	  [--secret postgres-db-creds] \
//	  my-postgres
//
// Target namespace is the current epinio-targeted namespace.
func NewServiceImportCmd(client ServicesService) *cobra.Command {
	var sourceNs, release, secret string

	cmd := &cobra.Command{
		Use:   "import SERVICE_NAME",
		Short: "Import an external Helm release as an Epinio service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.ServiceImport(sourceNs, release, secret, args[0])
		},
	}
	cmd.Flags().StringVar(&sourceNs, "source-namespace", "", "namespace of the external Helm release")
	cmd.Flags().StringVar(&release, "release", "", "Helm release name")
	cmd.Flags().StringVar(&secret, "secret", "", "credential secret name (optional; auto-detected when omitted)")
	_ = cmd.MarkFlagRequired("source-namespace")
	_ = cmd.MarkFlagRequired("release")

	return cmd
}

// NewServiceImportableCmd wires `epinio service importable SOURCE_NAMESPACE`.
func NewServiceImportableCmd(client ServicesService) *cobra.Command {
	return &cobra.Command{
		Use:   "importable SOURCE_NAMESPACE",
		Short: "List Helm releases in SOURCE_NAMESPACE that can be imported as services",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return client.ServiceImportable(args[0])
		},
	}
}
