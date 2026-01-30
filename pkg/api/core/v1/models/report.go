// Copyright © 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package models

// SystemPodReport represents a pod entry in the report header.
type SystemPodReport struct {
	Name     string `json:"name"`
	Ready    string `json:"ready"`
	Status   string `json:"status"`
	Restarts int32  `json:"restarts"`
	Age      string `json:"age"`
}

// NodeReport represents a single node row in the report.
type NodeReport struct {
	ID                      string `json:"id"`
	Address                 string `json:"address"`
	Etcd                    bool   `json:"etcd"`
	ControlPlane            bool   `json:"controlPlane"`
	Worker                  bool   `json:"worker"`
	CPU                     string `json:"cpu"`
	RAM                     string `json:"ram"`
	OS                      string `json:"os"`
	ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
	CreatedAt               string `json:"createdAt"`
}

// ClusterReport represents a cluster section in the report.
type ClusterReport struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	KubeVersion string       `json:"kubeVersion"`
	Provider    string       `json:"provider"`
	CreatedAt   string       `json:"createdAt"`
	Nodes       []NodeReport `json:"nodes"`
	NodeCount   int          `json:"nodeCount"`
}

// ReportResponse represents a rancher-like node report.
type ReportResponse struct {
	GeneratedAt       string            `json:"generatedAt"`
	GeneratedAtHuman  string            `json:"generatedAtHuman"`
	EpinioVersion     string            `json:"epinioVersion"`
	KubernetesVersion string            `json:"kubernetesVersion"`
	Platform          string            `json:"platform"`
	SystemPods        []SystemPodReport `json:"systemPods"`
	Clusters          []ClusterReport   `json:"clusters"`
	Applications      []AppScaleReport  `json:"applications"`
	TotalNodeCount    int               `json:"totalNodeCount"`
}

// AppScaleReport represents app creation and scaling metadata.
type AppScaleReport struct {
	Name             string `json:"name"`
	Namespace        string `json:"namespace"`
	CreatedAt        string `json:"createdAt"`
	DesiredInstances int32  `json:"desiredInstances"`
	LastScaleAt      string `json:"lastScaleAt,omitempty"`
	LastScaleBy      string `json:"lastScaleBy,omitempty"`
	LastScaleFrom    int32  `json:"lastScaleFrom,omitempty"`
	LastScaleTo      int32  `json:"lastScaleTo,omitempty"`
}
