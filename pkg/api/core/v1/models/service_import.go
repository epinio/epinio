// Copyright © 2026 - SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0

package models

// ReleaseCandidate is a Helm release discovered by the import-scan endpoint.
type ReleaseCandidate struct {
	Namespace string   `json:"namespace"`
	Release   string   `json:"release"`
	Chart     string   `json:"chart,omitempty"`
	Secrets   []string `json:"secrets"`
}

// ServiceImportRequest is the POST body for importing an external Helm
// release as an Epinio service. The target namespace comes from the URL.
type ServiceImportRequest struct {
	SourceNamespace string `json:"source_namespace"`
	Release         string `json:"release"`
	SourceSecret    string `json:"source_secret,omitempty"`
	ServiceName     string `json:"service_name"`
}

// ServiceImportScanResponse is the GET response for the scan endpoint.
type ServiceImportScanResponse struct {
	Candidates []ReleaseCandidate `json:"candidates"`
}
