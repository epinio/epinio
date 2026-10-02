// Copyright © 2021 - 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apibatchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// stubJobLister is a local JobLister so these tests can live in the internal
// package. applicationfakes imports application, so it cannot be used here.
type stubJobLister struct {
	jobs     *apibatchv1.JobList
	err      error
	selector string
}

func (s *stubJobLister) ListJobs(
	ctx context.Context,
	namespace, selector string,
) (*apibatchv1.JobList, error) {
	s.selector = selector
	if s.err != nil {
		return nil, s.err
	}
	return s.jobs, nil
}

func stagingJob(
	appName, namespace string,
	status v1.ConditionStatus,
	conditionType apibatchv1.JobConditionType,
) apibatchv1.Job {
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

var _ = Describe("stagingStatus", func() {
	const namespace = "some-namespace"

	It("reports a job without a terminal condition as active", func() {
		lister := &stubJobLister{jobs: &apibatchv1.JobList{
			Items: []apibatchv1.Job{
				stagingJob("app1", namespace, v1.ConditionFalse, apibatchv1.JobComplete),
			},
		}}

		infos, err := stagingStatus(context.Background(), lister, namespace, []string{"app1"})

		Expect(err).To(BeNil())
		Expect(infos).To(HaveLen(1))
		info := infos[EncodeConfigurationKey("app1", namespace)]
		Expect(string(info.status)).To(Equal(models.ApplicationStagingActive))
		Expect(info.completedAt).To(BeNil())
	})

	It("reports a completed job as done and records when it finished", func() {
		lister := &stubJobLister{jobs: &apibatchv1.JobList{
			Items: []apibatchv1.Job{
				stagingJob("app1", namespace, v1.ConditionTrue, apibatchv1.JobComplete),
			},
		}}

		infos, err := stagingStatus(context.Background(), lister, namespace, []string{"app1"})

		Expect(err).To(BeNil())
		info := infos[EncodeConfigurationKey("app1", namespace)]
		Expect(string(info.status)).To(Equal(models.ApplicationStagingDone))
		Expect(info.completedAt).ToNot(BeNil())
	})

	It("reports a failed job as failed and records when it finished", func() {
		lister := &stubJobLister{jobs: &apibatchv1.JobList{
			Items: []apibatchv1.Job{
				stagingJob("app1", namespace, v1.ConditionTrue, apibatchv1.JobFailed),
			},
		}}

		infos, err := stagingStatus(context.Background(), lister, namespace, []string{"app1"})

		Expect(err).To(BeNil())
		info := infos[EncodeConfigurationKey("app1", namespace)]
		Expect(string(info.status)).To(Equal(models.ApplicationStagingFailed))
		Expect(info.completedAt).ToNot(BeNil())
	})

	It("returns a status per app when several are named", func() {
		lister := &stubJobLister{jobs: &apibatchv1.JobList{
			Items: []apibatchv1.Job{
				stagingJob("app1", namespace, v1.ConditionFalse, apibatchv1.JobComplete),
				stagingJob("app2", namespace, v1.ConditionTrue, apibatchv1.JobComplete),
			},
		}}

		infos, err := stagingStatus(
			context.Background(), lister, namespace, []string{"app1", "app2"},
		)

		Expect(err).To(BeNil())
		Expect(infos).To(HaveLen(2))
		Expect(lister.selector).To(ContainSubstring("app.kubernetes.io/name in ("))
	})

	It("does not scope the selector when no app names are given", func() {
		lister := &stubJobLister{jobs: &apibatchv1.JobList{
			Items: []apibatchv1.Job{
				stagingJob("app1", namespace, v1.ConditionFalse, apibatchv1.JobComplete),
			},
		}}

		_, err := stagingStatus(context.Background(), lister, namespace, nil)

		Expect(err).To(BeNil())
		Expect(lister.selector).ToNot(ContainSubstring("in ("))
	})

	It("propagates a listing error", func() {
		lister := &stubJobLister{err: errors.New("k8s unavailable")}

		_, err := stagingStatus(context.Background(), lister, namespace, []string{"app1"})

		Expect(err).ToNot(BeNil())
	})
})
