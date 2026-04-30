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
	"math/rand"
	"time"

	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/application/applicationfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	apibatchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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
