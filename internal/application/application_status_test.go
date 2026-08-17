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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

var _ = Describe("assignApplicationStatus", func() {
	one := int32(1)
	zero := int32(0)

	It("marks active staging as staging", func() {
		app := &models.App{StagingStatus: models.ApplicationStagingActive}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationStaging)))
	})

	It("marks staging failure without workload as error", func() {
		app := &models.App{StagingStatus: models.ApplicationStagingFailed}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationError)))
		Expect(app.StatusMessage).To(Equal("staging failed"))
	})

	It("marks staging done without workload as deploying, not error", func() {
		app := &models.App{
			StagingStatus: models.ApplicationStagingDone,
			Configuration: models.ApplicationConfiguration{Instances: &one},
		}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationDeploying)))
		Expect(app.StatusMessage).To(Equal("deploying"))
	})

	It("marks staging done without workload as deploying when instances nil", func() {
		app := &models.App{
			StagingStatus: models.ApplicationStagingDone,
		}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationDeploying)))
		Expect(app.StatusMessage).To(Equal("deploying"))
	})

	It("keeps created when never staged", func() {
		app := &models.App{}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationCreated)))
	})

	It("keeps created when staging done and scaled to zero", func() {
		app := &models.App{
			StagingStatus: models.ApplicationStagingDone,
			Configuration: models.ApplicationConfiguration{Instances: &zero},
		}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationCreated)))
	})

	It("marks staging failed when stage id remains after staging job is gone", func() {
		app := &models.App{StageID: "abc123"}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationError)))
		Expect(app.StatusMessage).To(Equal("staging failed"))
	})

	It("marks deployment failed when stage id and image remain after staging job is gone", func() {
		app := &models.App{
			StageID:  "abc123",
			ImageURL: "registry.example/apps/ns-app:deadbeef",
		}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationError)))
		Expect(app.StatusMessage).To(Equal("deployment failed"))
	})

	It("keeps created when stage id remains but scaled to zero", func() {
		app := &models.App{
			StageID:       "abc123",
			Configuration: models.ApplicationConfiguration{Instances: &zero},
		}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationCreated)))
	})

	It("keeps running when workload exists even if staging failed", func() {
		app := &models.App{
			StagingStatus: models.ApplicationStagingFailed,
			Workload:      &models.AppDeployment{Active: true},
		}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationRunning)))
	})

	It("keeps running when workload exists after staging done", func() {
		app := &models.App{
			StagingStatus: models.ApplicationStagingDone,
			Workload:      &models.AppDeployment{Active: true},
		}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationRunning)))
	})

	It("marks error when workload pods are crash-looping", func() {
		app := &models.App{
			Configuration: models.ApplicationConfiguration{Instances: &one},
			Workload: &models.AppDeployment{
				Active:          true,
				DesiredReplicas: 1,
				ReadyReplicas:   0,
				Replicas: map[string]*models.PodInfo{
					"pod-1": {Restarts: 2, Ready: false},
				},
			},
		}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationError)))
		Expect(app.StatusMessage).To(Equal("deployment failed"))
	})

	It("keeps running when workload pods are starting without restarts", func() {
		app := &models.App{
			Configuration: models.ApplicationConfiguration{Instances: &one},
			Workload: &models.AppDeployment{
				Active:          true,
				DesiredReplicas: 1,
				ReadyReplicas:   0,
				Replicas: map[string]*models.PodInfo{
					"pod-1": {Restarts: 0, Ready: false},
				},
			},
		}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationRunning)))
	})

	It("preserves an existing status message on staging failure", func() {
		app := &models.App{
			StagingStatus: models.ApplicationStagingFailed,
			StatusMessage: "custom failure detail",
		}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationError)))
		Expect(app.StatusMessage).To(Equal("custom failure detail"))
	})

	It("preserves an existing status message while deploying", func() {
		app := &models.App{
			StagingStatus: models.ApplicationStagingDone,
			Configuration: models.ApplicationConfiguration{Instances: &one},
			StatusMessage: "rolling out",
		}
		assignApplicationStatus(app, nil)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationDeploying)))
		Expect(app.StatusMessage).To(Equal("rolling out"))
	})

	It("marks deployment failed when staging done long ago without workload", func() {
		completedAt := time.Now().Add(-3 * time.Minute)
		app := &models.App{
			StagingStatus: models.ApplicationStagingDone,
			ImageURL:      "registry.example/apps/ns-app:deadbeef",
			StageID:       "abc123",
			Configuration: models.ApplicationConfiguration{Instances: &one},
		}
		assignApplicationStatus(app, &completedAt)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationError)))
		Expect(app.StatusMessage).To(Equal("deployment failed"))
	})

	It("keeps deploying when staging recently completed without workload", func() {
		completedAt := time.Now().Add(-30 * time.Second)
		app := &models.App{
			StagingStatus: models.ApplicationStagingDone,
			ImageURL:      "registry.example/apps/ns-app:deadbeef",
			StageID:       "abc123",
			Configuration: models.ApplicationConfiguration{Instances: &one},
		}
		assignApplicationStatus(app, &completedAt)
		Expect(app.Status).To(Equal(models.ApplicationStatus(models.ApplicationDeploying)))
		Expect(app.StatusMessage).To(Equal("deploying"))
	})
})
