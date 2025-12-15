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

package application_test

import (
	"context"
	"time"

	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/application/applicationfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	apibatchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cache Cleanup", func() {
	var (
		ctx       context.Context
		jobLister *applicationfakes.FakeJobLister
	)

	BeforeEach(func() {
		ctx = context.Background()
		jobLister = &applicationfakes.FakeJobLister{}
	})

	Describe("GetLastBuildTime", func() {
		When("app has completed staging jobs", func() {
			It("returns the most recent job completion time", func() {
				appRef := models.NewAppRef("test-app", "test-ns")
				now := time.Now()
				olderTime := now.Add(-10 * time.Hour)
				newerTime := now.Add(-5 * time.Hour)

				jobList := &apibatchv1.JobList{
					Items: []apibatchv1.Job{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "job-1",
								Labels: map[string]string{
									"app.kubernetes.io/component": "staging",
									"app.kubernetes.io/name":      "test-app",
									"app.kubernetes.io/part-of":   "test-ns",
								},
							},
							Status: apibatchv1.JobStatus{
								CompletionTime: &metav1.Time{Time: olderTime},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "job-2",
								Labels: map[string]string{
									"app.kubernetes.io/component": "staging",
									"app.kubernetes.io/name":      "test-app",
									"app.kubernetes.io/part-of":   "test-ns",
								},
							},
							Status: apibatchv1.JobStatus{
								CompletionTime: &metav1.Time{Time: newerTime},
							},
						},
					},
				}

				jobLister.ListJobsReturns(jobList, nil)

				lastBuildTime, err := application.GetLastBuildTime(ctx, jobLister, appRef)
				Expect(err).ToNot(HaveOccurred())
				Expect(lastBuildTime).To(BeTemporally("~", newerTime, time.Second))
			})
		})

		When("app has no staging jobs", func() {
			It("returns zero time", func() {
				appRef := models.NewAppRef("test-app", "test-ns")
				jobList := &apibatchv1.JobList{
					Items: []apibatchv1.Job{},
				}

				jobLister.ListJobsReturns(jobList, nil)

				lastBuildTime, err := application.GetLastBuildTime(ctx, jobLister, appRef)
				Expect(err).ToNot(HaveOccurred())
				Expect(lastBuildTime.IsZero()).To(BeTrue())
			})
		})

		When("app has jobs but none are completed", func() {
			It("returns zero time", func() {
				appRef := models.NewAppRef("test-app", "test-ns")
				jobList := &apibatchv1.JobList{
					Items: []apibatchv1.Job{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "job-1",
								Labels: map[string]string{
									"app.kubernetes.io/component": "staging",
									"app.kubernetes.io/name":      "test-app",
									"app.kubernetes.io/part-of":   "test-ns",
								},
							},
							Status: apibatchv1.JobStatus{
								// No CompletionTime
							},
						},
					},
				}

				jobLister.ListJobsReturns(jobList, nil)

				lastBuildTime, err := application.GetLastBuildTime(ctx, jobLister, appRef)
				Expect(err).ToNot(HaveOccurred())
				Expect(lastBuildTime.IsZero()).To(BeTrue())
			})
		})
	})

	Describe("ParseCachePVCName", func() {
		When("PVC name follows the expected pattern with hash", func() {
			It("extracts app name and namespace correctly", func() {
				// Example: "test-ns-cache-myapp-abc123def456..." (40-char hash)
				// The actual hash would be 40 characters, but for testing we'll use a shorter one
				pvcName := "test-ns-cache-myapp-1234567890123456789012345678901234567890"

				appName, appNamespace := application.ParseCachePVCName(pvcName)

				Expect(appNamespace).To(Equal("test-ns"))
				Expect(appName).To(Equal("myapp"))
			})
		})

		When("PVC name has app name with dash before hash", func() {
			It("removes trailing dash from app name", func() {
				pvcName := "namespace-cache-app-name-1234567890123456789012345678901234567890"

				appName, appNamespace := application.ParseCachePVCName(pvcName)

				Expect(appNamespace).To(Equal("namespace"))
				Expect(appName).To(Equal("app-name"))
			})
		})

		When("PVC name doesn't contain -cache-", func() {
			It("returns empty strings", func() {
				pvcName := "some-other-pvc-name"

				appName, appNamespace := application.ParseCachePVCName(pvcName)

				Expect(appName).To(BeEmpty())
				Expect(appNamespace).To(BeEmpty())
			})
		})

		When("PVC name is too short", func() {
			It("returns empty strings", func() {
				pvcName := "ns-cache-app-123" // Too short for hash

				appName, appNamespace := application.ParseCachePVCName(pvcName)

				Expect(appName).To(BeEmpty())
				Expect(appNamespace).To(BeEmpty())
			})
		})

		When("PVC name starts with -cache-", func() {
			It("returns empty strings (no namespace)", func() {
				pvcName := "-cache-app-1234567890123456789012345678901234567890"

				appName, appNamespace := application.ParseCachePVCName(pvcName)

				Expect(appName).To(BeEmpty())
				Expect(appNamespace).To(BeEmpty())
			})
		})

		When("PVC name has no app name before hash", func() {
			It("returns empty strings", func() {
				pvcName := "namespace-cache-1234567890123456789012345678901234567890"

				appName, appNamespace := application.ParseCachePVCName(pvcName)

				// This should fail because there's no app name, just hash
				Expect(appName).To(BeEmpty())
				Expect(appNamespace).To(BeEmpty())
			})
		})
	})

	// Note: Additional test coverage needed (requires integration test environment or complex mocks):
	// - FindStaleCachePVCs with labeled PVCs
	// - FindStaleCachePVCs with unlabeled PVCs (name parsing)
	// - DeleteStaleCachePVCs deletion logic
	// - CleanupStaleCaches with checkAppExists filtering
	// - CleanupStaleCaches dry-run vs delete paths
	// - Error propagation in cleanup operations
	// These would require:
	//   - Mock Kubernetes client for PVC operations
	//   - Mock cluster for app existence checks
	//   - Full integration test environment
	// Consider adding these to the acceptance test suite.
})
