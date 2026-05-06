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
	"errors"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/application/applicationfakes"
	"github.com/epinio/epinio/internal/s3manager"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/go-logr/logr"

	apibatchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mockS3Manager is a test mock for s3manager.S3Manager
type mockS3Manager struct {
	deleteObjectFunc func(ctx context.Context, objectID string) error
	deletedObjects   []string
}

var _ s3manager.S3Manager = (*mockS3Manager)(nil)

func (m *mockS3Manager) Meta(ctx context.Context, blobUID string) (map[string]string, error) {
	return nil, nil
}

func (m *mockS3Manager) UploadStream(ctx context.Context, file io.Reader, size int64, metadata map[string]string) (string, error) {
	return "", nil
}

func (m *mockS3Manager) Upload(ctx context.Context, filepath string, metadata map[string]string) (string, error) {
	return "", nil
}

func (m *mockS3Manager) EnsureBucket(ctx context.Context) error {
	return nil
}

func (m *mockS3Manager) DeleteObject(ctx context.Context, objectID string) error {
	m.deletedObjects = append(m.deletedObjects, objectID)
	if m.deleteObjectFunc != nil {
		return m.deleteObjectFunc(ctx, objectID)
	}
	return nil
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate -header ../../LICENSE_HEADER . JobLister
type MockJobListerInterface struct {
	application.JobLister
}

var r *rand.Rand

var _ = Describe("application", func() {

	var fake *applicationfakes.FakeJobLister
	var namespace string

	BeforeEach(func() {
		fake = &applicationfakes.FakeJobLister{}

		r = rand.New(rand.NewSource(time.Now().UnixNano()))
		namespace = fmt.Sprintf("namespace-%d", r.Intn(1000))
	})

	appName := func() string {
		return fmt.Sprintf("appname-%d", r.Intn(1000))
	}

	Describe("IsCurrentlyStaging", func() {

		When("there are no jobs for the specified app", func() {
			It("returns false", func() {
				// setup a list of fake staging jobs
				stagingJobs := &apibatchv1.JobList{
					Items: []apibatchv1.Job{
						makeJob("another-app", namespace, v1.ConditionTrue, apibatchv1.JobComplete),
					},
				}
				fake.ListJobsReturns(stagingJobs, nil)

				isStaging, err := application.IsCurrentlyStaging(context.Background(), fake, namespace, appName())
				Expect(err).To(BeNil())
				Expect(isStaging).To(BeFalse())
			})
		})

		When("the staging job completed for the specified app", func() {
			It("returns true (app already staged)", func() {
				app := appName()

				// setup a list of fake staging jobs
				stagingJobs := &apibatchv1.JobList{
					Items: []apibatchv1.Job{
						makeJob("another-app", namespace, v1.ConditionTrue, apibatchv1.JobComplete),
						makeJob(app, namespace, v1.ConditionTrue, apibatchv1.JobComplete),
						makeJob("another-app2", namespace, v1.ConditionTrue, apibatchv1.JobComplete),
					},
				}
				fake.ListJobsReturns(stagingJobs, nil)

				isStaging, err := application.IsCurrentlyStaging(context.Background(), fake, namespace, app)
				Expect(err).To(BeNil())
				Expect(isStaging).To(BeFalse())
			})
		})

		When("the staging job failed for the specified app", func() {
			It("returns false", func() {
				app := appName()

				// setup a list of fake staging jobs
				stagingJobs := &apibatchv1.JobList{
					Items: []apibatchv1.Job{
						makeJob("another-app", namespace, v1.ConditionTrue, apibatchv1.JobComplete),
						makeJob("another-app2", namespace, v1.ConditionTrue, apibatchv1.JobComplete),
						makeJob(app, namespace, v1.ConditionTrue, apibatchv1.JobFailed),
					},
				}
				fake.ListJobsReturns(stagingJobs, nil)

				isStaging, err := application.IsCurrentlyStaging(context.Background(), fake, namespace, app)
				Expect(err).To(BeNil())
				Expect(isStaging).To(BeFalse())
			})
		})

		When("the staging job is still running", func() {
			It("returns true", func() {
				app := appName()

				// setup a list of fake staging jobs
				stagingJobs := &apibatchv1.JobList{
					Items: []apibatchv1.Job{
						makeJob("another-app", namespace, v1.ConditionTrue, apibatchv1.JobComplete),
						makeJob("another-app2", namespace, v1.ConditionTrue, apibatchv1.JobComplete),
						makeJob(app, namespace, v1.ConditionFalse, apibatchv1.JobFailed),
					},
				}
				fake.ListJobsReturns(stagingJobs, nil)

				isStaging, err := application.IsCurrentlyStaging(context.Background(), fake, namespace, app)
				Expect(err).To(BeNil())
				Expect(isStaging).To(BeTrue())
			})
		})

		When("the request for jobs failed", func() {
			It("returns an error", func() {
				fake.ListJobsReturns(nil, errors.New("something bad happened"))

				_, err := application.IsCurrentlyStaging(context.Background(), fake, namespace, "app")
				Expect(err).NotTo(BeNil())
			})
		})
	})

	Describe("StagingStatuses", func() {
		When("there is one job running for app1 and a completed job for app2", func() {
			It("returns true for app1 and false for app2", func() {
				app1, app2 := appName(), appName()

				// setup a list of fake staging jobs
				stagingJobs := &apibatchv1.JobList{
					Items: []apibatchv1.Job{
						makeJob(app1, namespace, v1.ConditionFalse, apibatchv1.JobComplete),
						makeJob(app2, namespace, v1.ConditionTrue, apibatchv1.JobComplete),
					},
				}
				fake.ListJobsReturns(stagingJobs, nil)

				isStagingMap, err := application.StagingStatuses(context.Background(), fake, namespace)
				Expect(err).To(BeNil())
				Expect(isStagingMap).To(HaveLen(2))
				Expect(string(isStagingMap[application.EncodeConfigurationKey(app1, namespace)])).To(Equal(models.ApplicationStagingActive))
				Expect(string(isStagingMap[application.EncodeConfigurationKey(app2, namespace)])).To(Equal(models.ApplicationStagingDone))
			})
		})
	})

	Describe("UnstageResult", func() {
		Describe("HasIncompleteCleanup", func() {
			When("no failed blob cleanups", func() {
				It("returns false", func() {
					result := &application.UnstageResult{}
					Expect(result.HasIncompleteCleanup()).To(BeFalse())
				})
			})

			When("there are failed blob cleanups", func() {
				It("returns true", func() {
					result := &application.UnstageResult{
						FailedBlobCleanups: []string{"blob-1", "blob-2"},
					}
					Expect(result.HasIncompleteCleanup()).To(BeTrue())
				})
			})
		})

		Describe("CleanupWarning", func() {
			When("no failed blob cleanups", func() {
				It("returns nil", func() {
					result := &application.UnstageResult{}
					Expect(result.CleanupWarning()).To(BeNil())
				})
			})

			When("there are failed blob cleanups", func() {
				It("returns an error wrapping ErrBlobCleanupIncomplete", func() {
					result := &application.UnstageResult{
						FailedBlobCleanups: []string{"blob-1", "blob-2"},
					}
					warning := result.CleanupWarning()
					Expect(warning).NotTo(BeNil())
					Expect(errors.Is(warning, application.ErrBlobCleanupIncomplete)).To(BeTrue())
					Expect(warning.Error()).To(ContainSubstring("2 blob(s) could not be deleted"))
					Expect(warning.Error()).To(ContainSubstring("blob-1"))
					Expect(warning.Error()).To(ContainSubstring("blob-2"))
				})
			})
		})
	})

	Describe("DeleteResult", func() {
		When("no warnings", func() {
			It("has empty warnings slice", func() {
				result := &application.DeleteResult{}
				Expect(result.Warnings).To(BeEmpty())
			})
		})

		When("there are warnings", func() {
			It("contains the warning messages", func() {
				result := &application.DeleteResult{
					Warnings: []string{"blob cleanup incomplete", "another warning"},
				}
				Expect(result.Warnings).To(HaveLen(2))
				Expect(result.Warnings[0]).To(ContainSubstring("blob cleanup incomplete"))
			})
		})
	})

	Describe("CleanupS3Blobs", func() {
		var (
			ctx       context.Context
			mockS3    *mockS3Manager
			appRef    models.AppRef
			nullLog   logr.Logger
		)

		BeforeEach(func() {
			ctx = context.Background()
			mockS3 = &mockS3Manager{}
			appRef = models.NewAppRef("test-app", "test-namespace")
			nullLog = logr.Discard() // Use a discarding logger for tests
		})

		makeJobWithBlob := func(appName, namespace, blobUID string) apibatchv1.Job {
			return apibatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":       appName,
						"app.kubernetes.io/part-of":    namespace,
						models.EpinioStageBlobUIDLabel: blobUID,
					},
				},
			}
		}

		When("all blobs delete successfully", func() {
			It("returns empty failed list and no error", func() {
				jobs := []apibatchv1.Job{
					makeJobWithBlob("test-app", "test-namespace", "blob-1"),
					makeJobWithBlob("test-app", "test-namespace", "blob-2"),
				}

				failed, err := application.CleanupS3Blobs(ctx, nullLog, mockS3, jobs, nil, appRef)

				Expect(err).To(BeNil())
				Expect(failed).To(BeEmpty())
				Expect(mockS3.deletedObjects).To(ConsistOf("blob-1", "blob-2"))
			})
		})

		When("a blob deletion fails with quota exceeded error", func() {
			It("continues with other deletions and returns failed blob in list", func() {
				jobs := []apibatchv1.Job{
					makeJobWithBlob("test-app", "test-namespace", "blob-1"),
					makeJobWithBlob("test-app", "test-namespace", "blob-2"),
					makeJobWithBlob("test-app", "test-namespace", "blob-3"),
				}

				mockS3.deleteObjectFunc = func(ctx context.Context, objectID string) error {
					if objectID == "blob-2" {
						return errors.New("QuotaExceeded: storage limit reached")
					}
					return nil
				}

				failed, err := application.CleanupS3Blobs(ctx, nullLog, mockS3, jobs, nil, appRef)

				Expect(err).To(BeNil())
				Expect(failed).To(ConsistOf("blob-2"))
				// All blobs should have been attempted
				Expect(mockS3.deletedObjects).To(ConsistOf("blob-1", "blob-2", "blob-3"))
			})
		})

		When("multiple blob deletions fail with quota exceeded errors", func() {
			It("returns all failed blobs in the list", func() {
				jobs := []apibatchv1.Job{
					makeJobWithBlob("test-app", "test-namespace", "blob-1"),
					makeJobWithBlob("test-app", "test-namespace", "blob-2"),
					makeJobWithBlob("test-app", "test-namespace", "blob-3"),
				}

				mockS3.deleteObjectFunc = func(ctx context.Context, objectID string) error {
					if objectID == "blob-1" || objectID == "blob-3" {
						return errors.New("quota exceeded on bucket")
					}
					return nil
				}

				failed, err := application.CleanupS3Blobs(ctx, nullLog, mockS3, jobs, nil, appRef)

				Expect(err).To(BeNil())
				Expect(failed).To(ConsistOf("blob-1", "blob-3"))
			})
		})

		When("a blob deletion fails with a non-quota error", func() {
			It("returns an error immediately and stops processing", func() {
				jobs := []apibatchv1.Job{
					makeJobWithBlob("test-app", "test-namespace", "blob-1"),
					makeJobWithBlob("test-app", "test-namespace", "blob-2"),
					makeJobWithBlob("test-app", "test-namespace", "blob-3"),
				}

				mockS3.deleteObjectFunc = func(ctx context.Context, objectID string) error {
					if objectID == "blob-2" {
						return errors.New("connection refused")
					}
					return nil
				}

				failed, err := application.CleanupS3Blobs(ctx, nullLog, mockS3, jobs, nil, appRef)

				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("connection refused"))
				Expect(err.Error()).To(ContainSubstring("blob-2"))
				Expect(failed).To(BeNil())
				// Should have attempted blob-1 and blob-2, but not blob-3
				Expect(mockS3.deletedObjects).To(ConsistOf("blob-1", "blob-2"))
			})
		})

		When("there is a current job", func() {
			It("skips the current job's blob", func() {
				jobs := []apibatchv1.Job{
					makeJobWithBlob("test-app", "test-namespace", "blob-1"),
					makeJobWithBlob("test-app", "test-namespace", "blob-current"),
					makeJobWithBlob("test-app", "test-namespace", "blob-3"),
				}
				currentJob := &jobs[1] // blob-current

				failed, err := application.CleanupS3Blobs(ctx, nullLog, mockS3, jobs, currentJob, appRef)

				Expect(err).To(BeNil())
				Expect(failed).To(BeEmpty())
				// Should NOT have deleted blob-current
				Expect(mockS3.deletedObjects).To(ConsistOf("blob-1", "blob-3"))
			})
		})

		When("Minio-style quota error occurs", func() {
			It("treats it as a non-fatal quota error", func() {
				jobs := []apibatchv1.Job{
					makeJobWithBlob("test-app", "test-namespace", "blob-1"),
				}

				mockS3.deleteObjectFunc = func(ctx context.Context, objectID string) error {
					return errors.New("Storage backend has reached its minimum free drive threshold")
				}

				failed, err := application.CleanupS3Blobs(ctx, nullLog, mockS3, jobs, nil, appRef)

				Expect(err).To(BeNil())
				Expect(failed).To(ConsistOf("blob-1"))
			})
		})

		When("no jobs to process", func() {
			It("returns empty results", func() {
				jobs := []apibatchv1.Job{}

				failed, err := application.CleanupS3Blobs(ctx, nullLog, mockS3, jobs, nil, appRef)

				Expect(err).To(BeNil())
				Expect(failed).To(BeEmpty())
				Expect(mockS3.deletedObjects).To(BeEmpty())
			})
		})
	})
})

func makeJob(appName, namespace string, status v1.ConditionStatus, conditionType apibatchv1.JobConditionType) apibatchv1.Job {
	return apibatchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app.kubernetes.io/name":    appName,
				"app.kubernetes.io/part-of": namespace,
			},
		},
		Status: apibatchv1.JobStatus{
			Conditions: []apibatchv1.JobCondition{
				{Status: status, Type: conditionType},
			},
		},
	}
}
