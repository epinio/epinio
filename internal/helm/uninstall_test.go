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

package helm

import (
	"fmt"

	hc "github.com/mittwald/go-helm-client"
	hcmock "github.com/mittwald/go-helm-client/mock"
	"go.uber.org/mock/gomock"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/release/common"
	helmrelease "helm.sh/helm/v4/pkg/release/v1"
	helmdriver "helm.sh/helm/v4/pkg/storage/driver"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("waitStrategyFor()", func() {
	It("maps wait=true to the watcher strategy", func() {
		Expect(waitStrategyFor(true)).To(Equal(kube.StatusWatcherStrategy))
	})

	It("maps wait=false to the hookOnly strategy, never an unset one", func() {
		// Helm v4 rejects an unset WaitStrategy outright ("wait strategy not set"),
		// unlike Helm v3 where an unset Wait simply meant "don't wait". hookOnly is the
		// mapping Helm's own --wait CLI flag uses for its non-waiting default.
		strategy := waitStrategyFor(false)
		Expect(strategy).To(Equal(kube.HookOnlyStrategy))
		Expect(strategy).ToNot(BeEmpty())
	})
})

var _ = Describe("appChartSpec()", func() {
	It("always waits for every resource and rolls back on failure", func() {
		// Helm v3's `Atomic: true` implied `Wait: true`. Helm v4 splits that into two
		// independent fields; both must be set to keep application deploys atomic.
		spec := appChartSpec("my-release", "my-chart", "1.2.3", "my-namespace", "values: yaml", true)

		Expect(spec.WaitStrategy).To(Equal(kube.StatusWatcherStrategy))
		Expect(spec.RollbackOnFailure).To(BeTrue())
		Expect(spec.ReleaseName).To(Equal("my-release"))
		Expect(spec.ChartName).To(Equal("my-chart"))
		Expect(spec.Version).To(Equal("1.2.3"))
		Expect(spec.Namespace).To(Equal("my-namespace"))
		Expect(spec.ValuesYaml).To(Equal("values: yaml"))
		Expect(spec.ReuseValues).To(BeTrue())
	})
})

var _ = Describe("uninstallRelease()", func() {
	var (
		mockCtrl   *gomock.Controller
		mockClient *hcmock.MockClient
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = hcmock.NewMockClient(mockCtrl)
	})

	It("goes through UninstallRelease, never UninstallReleaseByName", func() {
		// UninstallReleaseByName never sets a WaitStrategy on the underlying Helm v4
		// uninstall action, so it always fails with "wait strategy not set". Every
		// uninstall in this package must go through UninstallRelease instead, which
		// threads WaitStrategy from the ChartSpec. UninstallReleaseByName is deliberately
		// not stubbed on the mock: an accidental call fails the test via gomock, not
		// silently at runtime against a real cluster.
		mockClient.EXPECT().
			UninstallRelease(&hc.ChartSpec{
				ReleaseName:  "my-release",
				WaitStrategy: kube.HookOnlyStrategy,
			}).
			Return(nil)

		Expect(uninstallRelease(mockClient, "my-release", kube.HookOnlyStrategy)).To(Succeed())
	})

	It("passes the requested strategy through unchanged", func() {
		mockClient.EXPECT().
			UninstallRelease(&hc.ChartSpec{
				ReleaseName:  "my-release",
				WaitStrategy: kube.StatusWatcherStrategy,
			}).
			Return(nil)

		Expect(uninstallRelease(mockClient, "my-release", kube.StatusWatcherStrategy)).To(Succeed())
	})
})

var _ = Describe("cleanupReleaseIfNeeded()", func() {
	var (
		mockCtrl   *gomock.Controller
		mockClient *hcmock.MockClient
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = hcmock.NewMockClient(mockCtrl)
	})

	It("does nothing when the release does not exist", func() {
		mockClient.EXPECT().GetRelease("my-release").Return(nil, helmdriver.ErrReleaseNotFound)

		Expect(cleanupReleaseIfNeeded(mockClient, "my-release")).To(Succeed())
	})

	It("does nothing when the release is already deployed", func() {
		mockClient.EXPECT().GetRelease("my-release").Return(&helmrelease.Release{
			Info: &helmrelease.Info{Status: common.StatusDeployed},
		}, nil)

		Expect(cleanupReleaseIfNeeded(mockClient, "my-release")).To(Succeed())
	})

	It("uninstalls a release stuck pending-install, waiting for completion", func() {
		mockClient.EXPECT().GetRelease("my-release").Return(&helmrelease.Release{
			Info: &helmrelease.Info{Status: common.StatusPendingInstall},
		}, nil)

		// Unlike Remove/RemoveService's non-waiting uninstall, this one must wait: an
		// immediately following install would otherwise race the still-in-flight deletion.
		mockClient.EXPECT().
			UninstallRelease(&hc.ChartSpec{
				ReleaseName:  "my-release",
				WaitStrategy: kube.StatusWatcherStrategy,
			}).
			Return(nil)

		Expect(cleanupReleaseIfNeeded(mockClient, "my-release")).To(Succeed())
	})

	It("recovers when uninstall errors but the release is actually gone", func() {
		mockClient.EXPECT().GetRelease("my-release").Return(&helmrelease.Release{
			Info: &helmrelease.Info{Status: common.StatusPendingInstall},
		}, nil)
		mockClient.EXPECT().UninstallRelease(gomock.Any()).Return(fmt.Errorf("boom"))
		mockClient.EXPECT().GetRelease("my-release").Return(nil, helmdriver.ErrReleaseNotFound)

		Expect(cleanupReleaseIfNeeded(mockClient, "my-release")).To(Succeed())
	})

	It("surfaces the uninstall error when the release still exists afterward", func() {
		mockClient.EXPECT().GetRelease("my-release").Return(&helmrelease.Release{
			Info: &helmrelease.Info{Status: common.StatusPendingInstall},
		}, nil)
		mockClient.EXPECT().UninstallRelease(gomock.Any()).Return(fmt.Errorf("boom"))
		mockClient.EXPECT().GetRelease("my-release").Return(&helmrelease.Release{
			Info: &helmrelease.Info{Status: common.StatusPendingInstall},
		}, nil)

		err := cleanupReleaseIfNeeded(mockClient, "my-release")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("boom"))
	})
})
