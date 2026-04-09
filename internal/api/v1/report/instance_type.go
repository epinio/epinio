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

package report

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

// instanceTypeVCPUs maps cloud instance type names to vCPU count for reporting.
// When a node has node.kubernetes.io/instance-type we use this for the "vCPU" column;
// otherwise we use the node's CPU capacity (which on VMs is typically vCPU as seen by the guest).
// Sources: AWS, Azure, GCP instance type docs. Subset of common types; unknown types fall back to node capacity.
var instanceTypeVCPUs = map[string]int{
	// AWS - General purpose (m5, m6, m6i, m7i)
	"m5.large": 2, "m5.xlarge": 4, "m5.2xlarge": 8, "m5.4xlarge": 16, "m5.8xlarge": 32, "m5.12xlarge": 48, "m5.16xlarge": 64, "m5.24xlarge": 96,
	"m6i.large": 2, "m6i.xlarge": 4, "m6i.2xlarge": 8, "m6i.4xlarge": 16, "m6i.8xlarge": 32, "m6i.12xlarge": 48, "m6i.16xlarge": 64, "m6i.24xlarge": 96, "m6i.32xlarge": 128,
	"m6a.large": 2, "m6a.xlarge": 4, "m6a.2xlarge": 8, "m6a.4xlarge": 16, "m6a.8xlarge": 32, "m6a.12xlarge": 48, "m6a.16xlarge": 64, "m6a.24xlarge": 96, "m6a.32xlarge": 128,
	"m7i.large": 2, "m7i.xlarge": 4, "m7i.2xlarge": 8, "m7i.4xlarge": 16, "m7i.8xlarge": 32, "m7i.12xlarge": 48, "m7i.16xlarge": 64, "m7i.24xlarge": 96, "m7i.48xlarge": 192,
	// AWS - Burstable (t3, t3a)
	"t3.micro": 2, "t3.small": 2, "t3.medium": 2, "t3.large": 2, "t3.xlarge": 4, "t3.2xlarge": 8,
	"t3a.micro": 2, "t3a.small": 2, "t3a.medium": 2, "t3a.large": 2, "t3a.xlarge": 4, "t3a.2xlarge": 8,
	// AWS - Compute optimized (c5, c6i)
	"c5.large": 2, "c5.xlarge": 4, "c5.2xlarge": 8, "c5.4xlarge": 16, "c5.9xlarge": 36, "c5.12xlarge": 48, "c5.18xlarge": 72, "c5.24xlarge": 96,
	"c6i.large": 2, "c6i.xlarge": 4, "c6i.2xlarge": 8, "c6i.4xlarge": 16, "c6i.8xlarge": 32, "c6i.12xlarge": 48, "c6i.16xlarge": 64, "c6i.24xlarge": 96, "c6i.32xlarge": 128,
	// Azure - General purpose (D, E series)
	"Standard_D2s_v3": 2, "Standard_D4s_v3": 4, "Standard_D8s_v3": 8, "Standard_D16s_v3": 16, "Standard_D32s_v3": 32, "Standard_D48s_v3": 48, "Standard_D64s_v3": 64,
	"Standard_D2as_v4": 2, "Standard_D4as_v4": 4, "Standard_D8as_v4": 8, "Standard_D16as_v4": 16, "Standard_D32as_v4": 32, "Standard_D48as_v4": 48, "Standard_D64as_v4": 64, "Standard_D96as_v4": 96,
	"Standard_E2s_v3": 2, "Standard_E4s_v3": 4, "Standard_E8s_v3": 8, "Standard_E16s_v3": 16, "Standard_E32s_v3": 32, "Standard_E48s_v3": 48, "Standard_E64s_v3": 64,
	"Standard_E2as_v4": 2, "Standard_E4as_v4": 4, "Standard_E8as_v4": 8, "Standard_E16as_v4": 16, "Standard_E32as_v4": 32, "Standard_E48as_v4": 48, "Standard_E64as_v4": 64, "Standard_E96as_v4": 96,
	// GCP - n1, n2 standard
	"n1-standard-1": 1, "n1-standard-2": 2, "n1-standard-4": 4, "n1-standard-8": 8, "n1-standard-16": 16, "n1-standard-32": 32, "n1-standard-64": 64, "n1-standard-96": 96,
	"n2-standard-2": 2, "n2-standard-4": 4, "n2-standard-8": 8, "n2-standard-16": 16, "n2-standard-32": 32, "n2-standard-48": 48, "n2-standard-64": 64, "n2-standard-80": 80, "n2-standard-128": 128,
	"e2-medium": 1, "e2-standard-2": 2, "e2-standard-4": 4, "e2-standard-8": 8, "e2-standard-16": 16, "e2-standard-32": 32,
}

const instanceTypeLabel = "node.kubernetes.io/instance-type"

// nodeVCPU returns the vCPU count to display for the node: from instance-type lookup when
// available, otherwise from node capacity (which on VMs is typically the guest vCPU count).
func nodeVCPU(node corev1.Node) string {
	if labels := node.Labels; labels != nil {
		if it := labels[instanceTypeLabel]; it != "" {
			if n, ok := instanceTypeVCPUs[it]; ok {
				return strconv.Itoa(n)
			}
		}
	}
	// Fallback: node capacity. Kubernetes reports in cores; on VMs this is usually vCPU.
	q := node.Status.Capacity.Cpu()
	if q == nil {
		return "0"
	}
	// Prefer integer when whole number
	if q.Value() == int64(q.MilliValue()/1000) {
		return strconv.FormatInt(q.Value(), 10)
	}
	return q.String()
}

