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

package helm_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/epinio/epinio/internal/helm"
	"github.com/golang/mock/gomock"
	hc "github.com/mittwald/go-helm-client"
	hcmock "github.com/mittwald/go-helm-client/mock"
	"helm.sh/helm/v3/pkg/release"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("SynchronizedClient", func() {
	var (
		mockCtrl   *gomock.Controller
		mockClient *hcmock.MockClient
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = hcmock.NewMockClient(mockCtrl)
	})

	setupMockRelease := func(ctx context.Context, releaseName string, duration time.Duration) *hc.ChartSpec {
		fakeRelease := &hc.ChartSpec{ReleaseName: releaseName}

		mockClient.
			EXPECT().
			InstallOrUpgradeChart(ctx, fakeRelease, nil).
			DoAndReturn(func(ctx context.Context, spec *hc.ChartSpec, opts *hc.GenericHelmOptions) (*release.Release, error) {
				time.Sleep(duration)
				return &release.Release{Name: spec.ReleaseName}, nil
			}).
			AnyTimes()

		return fakeRelease
	}

	It("should wait for releases in the same namespace", func() {
		ctx := context.Background()
		namespace := "namespace"

		// setup the mock with a couple of releases

		// release2s will take 2s
		release2s := setupMockRelease(ctx, "release-2s", 2*time.Second)
		// release3s will take 3s
		release3s := setupMockRelease(ctx, "release-3s", 3*time.Second)

		// create a synch client
		syncClient, err := helm.NewNamespaceSynchronizedHelmClient(namespace, mockClient)
		Expect(err).To(BeNil())

		// Begin TEST!
		wg := &sync.WaitGroup{}

		// let's see how long the two installation are taking
		// since they are done in the same namespace they should take 2s + 3s
		start := time.Now()

		// release2s will take 2s
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer GinkgoRecover()

			syncClient.InstallOrUpgradeChart(ctx, release2s, nil)
			fmt.Fprintln(GinkgoWriter, "done release2s")
			wg.Done()
		}(wg)

		// and release3s this will take 3s
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer GinkgoRecover()

			syncClient.InstallOrUpgradeChart(ctx, release3s, nil)
			fmt.Fprintln(GinkgoWriter, "done release3s")
			wg.Done()
		}(wg)

		wg.Wait()

		// at the end the elapsed time should be greater than 5s!
		elapsed := time.Since(start)
		Expect(elapsed).To(BeNumerically(">=", 5*time.Second))
	})

	FIt("should not wait for releases in different namespaces and to them concurrently", func() {
		ctx := context.Background()
		namespace1 := "namespace1"
		namespace2 := "namespace2"

		// setup the mock with a couple of releases

		// releaseOne3s will take 3s
		releaseOne3s := setupMockRelease(ctx, "release-ns1-3s", 3*time.Second)
		// releaseTwo3s will take 3s
		releaseTwo3s := setupMockRelease(ctx, "release-ns2-3s", 3*time.Second)

		// create two sync client
		syncClient1, err := helm.NewNamespaceSynchronizedHelmClient(namespace1, mockClient)
		Expect(err).To(BeNil())
		syncClient2, err := helm.NewNamespaceSynchronizedHelmClient(namespace2, mockClient)
		Expect(err).To(BeNil())

		// Begin TEST!
		wg := &sync.WaitGroup{}

		// let's see how long the two installation are taking
		// since they are done in the same namespace they should take 2s + 3s
		start := time.Now()

		// releaseOne3s will take 3s
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer GinkgoRecover()

			syncClient1.InstallOrUpgradeChart(ctx, releaseOne3s, nil)
			fmt.Fprintln(GinkgoWriter, "done releaseOne3s")
			wg.Done()
		}(wg)

		// and releaseTwo3s this will take 3s
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer GinkgoRecover()

			syncClient2.InstallOrUpgradeChart(ctx, releaseTwo3s, nil)
			fmt.Fprintln(GinkgoWriter, "done releaseTwo3s")
			wg.Done()
		}(wg)

		wg.Wait()

		// at the end the elapsed time should be greater than 3s but less than 4s!
		elapsed := time.Since(start)
		Expect(elapsed).To(BeNumerically(">=", 3*time.Second))
		Expect(elapsed).To(BeNumerically("<", 4*time.Second))
	})

	FIt("should not wait for releases in the same namespace and to them concurrently, while waiting for the same", func() {
		ctx := context.Background()
		namespace1 := "namespace1"
		namespace2 := "namespace2"

		syncClient1, err := helm.NewNamespaceSynchronizedHelmClient(namespace1, mockClient)
		Expect(err).To(BeNil())

		syncClient2, err := helm.NewNamespaceSynchronizedHelmClient(namespace2, mockClient)
		Expect(err).To(BeNil())

		// setup the mock with a couple of releases

		// releaseFoo2s will take 2s
		releaseFoo2s := setupMockRelease(ctx, "release-foo-2s", 2*time.Second)
		// releaseBar2s will take 2s
		releaseBar2s := setupMockRelease(ctx, "release-bar-2s", 2*time.Second)
		// release3s will take 3s
		release3s := setupMockRelease(ctx, "release-3s", 3*time.Second)

		// Begin TEST!
		wg := &sync.WaitGroup{}

		// let's see how long the two installation are taking
		// since they are done in the same namespace they should take 2s + 3s
		start := time.Now()

		// releaseFoo2s will take 2s
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer GinkgoRecover()

			fmt.Fprintln(GinkgoWriter, "installing releaseFoo2s")
			syncClient1.InstallOrUpgradeChart(ctx, releaseFoo2s, nil)
			fmt.Fprintln(GinkgoWriter, "done releaseFoo2s")
			wg.Done()
		}(wg)

		// releaseBar2s will take 2s
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer GinkgoRecover()

			fmt.Fprintln(GinkgoWriter, "installing releaseBar2s")
			syncClient1.InstallOrUpgradeChart(ctx, releaseBar2s, nil)
			fmt.Fprintln(GinkgoWriter, "done releaseBar2s")
			wg.Done()
		}(wg)

		// and release3s will take 3s
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer GinkgoRecover()

			fmt.Fprintln(GinkgoWriter, "installing release3s")
			syncClient2.InstallOrUpgradeChart(ctx, release3s, nil)
			fmt.Fprintln(GinkgoWriter, "done release3s")
			wg.Done()
		}(wg)

		wg.Wait()

		// at the end the elapsed time should be greater than 4s but less than 5s!
		elapsed := time.Since(start)
		Expect(elapsed).To(BeNumerically(">=", 4*time.Second))
		Expect(elapsed).To(BeNumerically("<", 5*time.Second))
	})
})
