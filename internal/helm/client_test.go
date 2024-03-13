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
	"sync"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/internal/helm"
	hc "github.com/mittwald/go-helm-client"
	hcmock "github.com/mittwald/go-helm-client/mock"
	"go.uber.org/mock/gomock"
	"helm.sh/helm/v3/pkg/release"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type CtxDuration struct{}

var _ = Describe("SynchronizedClient", func() {
	var (
		ctx        context.Context
		mockCtrl   *gomock.Controller
		mockClient *hcmock.MockClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = hcmock.NewMockClient(mockCtrl)
	})

	// setupMockRelease will setup the mock. This mock will look into the context for the CtxDuration{} key
	// to get the time.Duration used to time.Sleep() faking the release.
	// This was needed to register different InstallOrUpgradeChart() calls (thanks to the difference in the Context)
	setupMockRelease := func(ctx context.Context, myRelease *hc.ChartSpec) {
		mockClient.
			EXPECT().
			InstallOrUpgradeChart(ctx, myRelease, nil).
			DoAndReturn(func(ctx context.Context, spec *hc.ChartSpec, opts *hc.GenericHelmOptions) (*release.Release, error) {
				duration, _ := ctx.Value(CtxDuration{}).(time.Duration)
				time.Sleep(duration)
				return &release.Release{Name: spec.ReleaseName}, nil
			}).
			AnyTimes()
	}

	When("getting a namespace synchronized client", func() {

		It("should return the same client for the same namespace", func() {
			namespace := catalog.NewNamespaceName()

			syncClient1, err := helm.GetNamespaceSynchronizedHelmClient(namespace, mockClient)
			Expect(err).To(BeNil())

			syncClient2, err := helm.GetNamespaceSynchronizedHelmClient(namespace, mockClient)
			Expect(err).To(BeNil())

			Expect(syncClient1).To(Equal(syncClient2))
		})

		It("should return a different client for different namespaces", func() {
			namespace1 := catalog.NewNamespaceName()
			namespace2 := catalog.NewNamespaceName()

			syncClient1, err := helm.GetNamespaceSynchronizedHelmClient(namespace1, mockClient)
			Expect(err).To(BeNil())

			syncClient2, err := helm.GetNamespaceSynchronizedHelmClient(namespace2, mockClient)
			Expect(err).To(BeNil())

			Expect(syncClient1).To(Not(Equal(syncClient2)))
		})
	})

	When("installing or upgrading chart", func() {

		It("should wait for the same release in the same namespace", func() {
			// setup the mock with the same release, twice

			ctx2s := context.WithValue(ctx, CtxDuration{}, 2*time.Second)
			ctx3s := context.WithValue(ctx, CtxDuration{}, 3*time.Second)
			myRelease := &hc.ChartSpec{ReleaseName: "my-release"}

			setupMockRelease(ctx2s, myRelease)
			setupMockRelease(ctx3s, myRelease)

			// create a sync client
			namespace := catalog.NewNamespaceName()
			syncClient, err := helm.GetNamespaceSynchronizedHelmClient(namespace, mockClient)
			Expect(err).To(BeNil())

			// Begin TEST!

			wg := &sync.WaitGroup{}

			// let's see how long the two installations are taking
			// since they are done in the same namespace and with the same release they should take 5s
			start := time.Now()

			// myRelease will take 2s the first time
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer GinkgoRecover()

				syncClient.InstallOrUpgradeChart(ctx2s, myRelease, nil)
				wg.Done()
			}(wg)

			// second installation of myRelease will take 3s
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer GinkgoRecover()

				syncClient.InstallOrUpgradeChart(ctx3s, myRelease, nil)
				wg.Done()
			}(wg)

			wg.Wait()

			// the elapsed time should be greater than 5s
			elapsed := time.Since(start)
			Expect(elapsed).To(BeNumerically(">=", 5*time.Second))
		})

		It("should not wait for different releases in the same namespace", func() {
			// setup the mock with a couple of releases

			// release2s will take 2s
			ctx2s := context.WithValue(ctx, CtxDuration{}, 2*time.Second)
			release2s := &hc.ChartSpec{ReleaseName: "release-2s"}
			setupMockRelease(ctx2s, release2s)

			// release3s will take 3s
			ctx3s := context.WithValue(ctx, CtxDuration{}, 3*time.Second)
			release3s := &hc.ChartSpec{ReleaseName: "release-3s"}
			setupMockRelease(ctx3s, release3s)

			// create a sync client
			namespace := catalog.NewNamespaceName()
			syncClient, err := helm.GetNamespaceSynchronizedHelmClient(namespace, mockClient)
			Expect(err).To(BeNil())

			// Begin TEST!

			wg := &sync.WaitGroup{}

			// let's see how long the two installations are taking
			// since they are done in the same namespace but they are different they should take 3s
			start := time.Now()

			// release2s will take 2s
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer GinkgoRecover()

				syncClient.InstallOrUpgradeChart(ctx2s, release2s, nil)
				wg.Done()
			}(wg)

			// release3s will take 3s
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer GinkgoRecover()

				syncClient.InstallOrUpgradeChart(ctx3s, release3s, nil)
				wg.Done()
			}(wg)

			wg.Wait()

			// the elapsed time should be greater than 5s
			elapsed := time.Since(start)
			Expect(elapsed).To(BeNumerically(">=", 3*time.Second))
			Expect(elapsed).To(BeNumerically("<", 4*time.Second))
		})

		It("should not wait for releases in different namespaces and do them concurrently", func() {
			// setup the mock with a couple of releases with the same 3s duration

			ctx3s := context.WithValue(ctx, CtxDuration{}, 3*time.Second)

			// releaseOne3s will take 3s in the first namespace
			releaseOne3s := &hc.ChartSpec{ReleaseName: "release-ns1-3s"}
			setupMockRelease(ctx3s, releaseOne3s)

			// releaseOne3s will take 3s in the second namespace
			releaseTwo3s := &hc.ChartSpec{ReleaseName: "release-ns2-3s"}
			setupMockRelease(ctx3s, releaseTwo3s)

			// create two sync clients
			namespace1 := catalog.NewNamespaceName()
			syncClient1, err := helm.GetNamespaceSynchronizedHelmClient(namespace1, mockClient)
			Expect(err).To(BeNil())

			namespace2 := catalog.NewNamespaceName()
			syncClient2, err := helm.GetNamespaceSynchronizedHelmClient(namespace2, mockClient)
			Expect(err).To(BeNil())

			// Begin TEST!
			wg := &sync.WaitGroup{}

			// let's see how long the two installations are taking
			// since they are done in two different namespaces they should take 3s in total
			// because they will be done concurrently
			start := time.Now()

			// releaseOne3s will take 3s
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer GinkgoRecover()

				syncClient1.InstallOrUpgradeChart(ctx3s, releaseOne3s, nil)
				wg.Done()
			}(wg)

			// releaseTwo3s will take 3s
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer GinkgoRecover()

				syncClient2.InstallOrUpgradeChart(ctx3s, releaseTwo3s, nil)
				wg.Done()
			}(wg)

			wg.Wait()

			// at the end the elapsed time should be greater than 3s but less than 4s!
			elapsed := time.Since(start)
			Expect(elapsed).To(BeNumerically(">=", 3*time.Second))
			Expect(elapsed).To(BeNumerically("<", 4*time.Second))
		})

		It("should not wait for releases in different namespaces and do them concurrently, while waiting for the one in the same", func() {
			// setup the mock with three releases

			// release1s will take 1s
			ctx1s := context.WithValue(ctx, CtxDuration{}, 1*time.Second)
			release1s := &hc.ChartSpec{ReleaseName: "release-1s"}
			setupMockRelease(ctx1s, release1s)

			// release3s will take 3s
			ctx3s := context.WithValue(ctx, CtxDuration{}, 3*time.Second)
			release3s := &hc.ChartSpec{ReleaseName: "release-3s"}
			setupMockRelease(ctx3s, release3s)

			// release5s will take 5s
			ctx5s := context.WithValue(ctx, CtxDuration{}, 5*time.Second)
			release5s := &hc.ChartSpec{ReleaseName: "release-5s"}
			setupMockRelease(ctx5s, release5s)

			// create two sync clients
			namespace1 := catalog.NewNamespaceName()
			syncClient1, err := helm.GetNamespaceSynchronizedHelmClient(namespace1, mockClient)
			Expect(err).To(BeNil())

			namespace2 := catalog.NewNamespaceName()
			syncClient2, err := helm.GetNamespaceSynchronizedHelmClient(namespace2, mockClient)
			Expect(err).To(BeNil())

			// Begin TEST!
			wg := &sync.WaitGroup{}

			// let's see how long the installations will take
			// since the first two are done in the same namespace they should take 1s + 3s
			// and the other one whould take 5s. Total should be 5s!
			start := time.Now()

			// release1s will take 1s in NS 1
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer GinkgoRecover()

				syncClient1.InstallOrUpgradeChart(ctx1s, release1s, nil)
				wg.Done()
			}(wg)

			// release3s will take 3s in NS 1
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer GinkgoRecover()

				syncClient1.InstallOrUpgradeChart(ctx3s, release3s, nil)
				wg.Done()
			}(wg)

			// and release5s will take 5s in NS 2
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer GinkgoRecover()

				syncClient2.InstallOrUpgradeChart(ctx5s, release5s, nil)
				wg.Done()
			}(wg)

			wg.Wait()

			// at the end the elapsed time should be greater than 4s but less than 6s!
			elapsed := time.Since(start)
			Expect(elapsed).To(BeNumerically(">=", 4*time.Second))
			Expect(elapsed).To(BeNumerically("<", 6*time.Second))
		})
	})
})
