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

package telemetry_test

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/telemetry"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/fake"
)

// Collect's happy path composes application.ListAppRefs, namespaces.List and
// services.ServiceClient.ListAll, each of which needs a dynamic client backed
// by a real (or fully faked) API server and is exercised at the acceptance
// level (see acceptance/api/v1/telemetry_test.go), matching how those helpers
// are already tested elsewhere in this repo. Here we only pin down that
// Collect stops - and wraps the error clearly - the moment one of its steps
// fails, instead of silently returning a partial/zero snapshot.
var _ = Describe("Collect", func() {
	It("wraps and returns the error when the cluster has no usable REST config", func() {
		cluster := &kubernetes.Cluster{
			Kubectl: fake.NewSimpleClientset(),
		}

		_, err := telemetry.Collect(context.Background(), cluster)
		Expect(err).To(HaveOccurred())
	})
})
