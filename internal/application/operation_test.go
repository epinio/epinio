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
	"time"

	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OperationRegistry", func() {
	var (
		reg   *application.OperationRegistry
		clock time.Time
		ids   int
		app   models.AppRef
		ctx   = context.Background()
	)

	BeforeEach(func() {
		clock = time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
		ids = 0
		app = models.NewAppRef("app", "ns")
		reg = application.NewOperationRegistry(
			time.Minute,
			func() time.Time { return clock },
			func() (string, error) {
				ids++
				return fmt.Sprint("op", ids), nil
			},
		)
	})

	It("starts a pending operation", func() {
		op, _, err := reg.Start(ctx, app, models.OperationBuild)
		Expect(err).ToNot(HaveOccurred())
		Expect(op.ID).To(Equal("op1"))

		got, ok := reg.Get(app)
		Expect(ok).To(BeTrue())
		Expect(got.Phase).To(Equal(models.OperationPending))
		Expect(got.Kind).To(Equal(models.OperationBuild))
		Expect(got.App).To(Equal(app))
		Expect(got.StartedAt).To(Equal("2026-11-05T00:00:00Z"))
	})

	It("rejects a second start while one is running", func() {
		_, _, err := reg.Start(ctx, app, models.OperationBuild)
		Expect(err).ToNot(HaveOccurred())

		_, _, err = reg.Start(ctx, app, models.OperationDeploy)
		Expect(err).To(MatchError(application.ErrOperationInProgress))

		other := models.NewAppRef("other", "ns")
		_, _, err = reg.Start(ctx, other, models.OperationDeploy)
		Expect(err).ToNot(HaveOccurred())
	})

	It("allows a new start once the previous one finished", func() {
		first, _, err := reg.Start(ctx, app, models.OperationBuild)
		Expect(err).ToNot(HaveOccurred())
		reg.Finish(app, first.ID, nil)

		second, _, err := reg.Start(ctx, app, models.OperationDeploy)
		Expect(err).ToNot(HaveOccurred())
		Expect(second.ID).ToNot(Equal(first.ID))

		got, _ := reg.Get(app)
		Expect(got.ID).To(Equal(second.ID))
		Expect(got.Phase).To(Equal(models.OperationPending))
	})

	It("applies updates to the current operation", func() {
		op, _, _ := reg.Start(ctx, app, models.OperationBuild)
		reg.Update(app, op.ID, func(o *models.AppOperation) {
			o.Phase = models.OperationStaging
			o.StageID = "stage1"
		})

		got, _ := reg.Get(app)
		Expect(got.Phase).To(Equal(models.OperationStaging))
		Expect(got.StageID).To(Equal("stage1"))
	})

	It("drops updates from a superseded operation", func() {
		first, _, _ := reg.Start(ctx, app, models.OperationBuild)
		reg.Finish(app, first.ID, nil)
		second, _, _ := reg.Start(ctx, app, models.OperationDeploy)

		reg.Update(app, first.ID, func(o *models.AppOperation) {
			o.Phase = models.OperationFailed
		})
		reg.Finish(app, first.ID, errors.New("late"))

		got, _ := reg.Get(app)
		Expect(got.ID).To(Equal(second.ID))
		Expect(got.Phase).To(Equal(models.OperationPending))
		Expect(got.Error).To(BeEmpty())
	})

	It("ignores updates for an unknown application", func() {
		Expect(func() {
			reg.Update(app, "op1", func(o *models.AppOperation) {})
			reg.Finish(app, "op1", nil)
		}).ToNot(Panic())

		_, ok := reg.Get(app)
		Expect(ok).To(BeFalse())
	})

	It("records the error on a failed operation", func() {
		op, _, _ := reg.Start(ctx, app, models.OperationDeploy)
		clock = clock.Add(30 * time.Second)
		reg.Finish(app, op.ID, errors.New("helm timed out"))

		got, _ := reg.Get(app)
		Expect(got.Phase).To(Equal(models.OperationFailed))
		Expect(got.Error).To(Equal("helm timed out"))
		Expect(got.FinishedAt).To(Equal("2026-11-05T00:00:30Z"))
		Expect(got.Done()).To(BeTrue())
	})

	It("expires a finished operation after the TTL", func() {
		op, _, _ := reg.Start(ctx, app, models.OperationBuild)
		reg.Finish(app, op.ID, nil)

		clock = clock.Add(59 * time.Second)
		_, ok := reg.Get(app)
		Expect(ok).To(BeTrue())

		clock = clock.Add(2 * time.Second)
		_, ok = reg.Get(app)
		Expect(ok).To(BeFalse())
	})

	It("never expires a running operation", func() {
		_, _, err := reg.Start(ctx, app, models.OperationBuild)
		Expect(err).ToNot(HaveOccurred())

		clock = clock.Add(24 * time.Hour)
		got, ok := reg.Get(app)
		Expect(ok).To(BeTrue())
		Expect(got.Done()).To(BeFalse())
	})

	It("returns a copy the caller cannot change", func() {
		op, _, _ := reg.Start(ctx, app, models.OperationDeploy)
		reg.Update(app, op.ID, func(o *models.AppOperation) {
			o.Routes = []string{"a.example.com"}
		})

		got, _ := reg.Get(app)
		got.Phase = models.OperationFailed
		got.Routes[0] = "changed"

		again, _ := reg.Get(app)
		Expect(again.Phase).To(Equal(models.OperationPending))
		Expect(again.Routes).To(Equal([]string{"a.example.com"}))
	})

	It("cancels the runner context when the operation finishes", func() {
		op, opCtx, err := reg.Start(ctx, app, models.OperationBuild)
		Expect(err).ToNot(HaveOccurred())
		Expect(opCtx.Err()).ToNot(HaveOccurred())

		reg.Finish(app, op.ID, nil)
		Expect(opCtx.Err()).To(MatchError(context.Canceled))
	})

	It("cancels a running operation when the app is deleted", func() {
		_, opCtx, err := reg.Start(ctx, app, models.OperationDeploy)
		Expect(err).ToNot(HaveOccurred())

		reg.Delete(app)
		Expect(opCtx.Err()).To(MatchError(context.Canceled))
	})

	It("does not cancel the new operation when an old runner finishes",
		func() {
			first, _, _ := reg.Start(ctx, app, models.OperationBuild)
			reg.Delete(app)
			_, secondCtx, err := reg.Start(ctx, app, models.OperationBuild)
			Expect(err).ToNot(HaveOccurred())

			reg.Finish(app, first.ID, errors.New("canceled"))
			Expect(secondCtx.Err()).ToNot(HaveOccurred())
		})

	It("reports busy only while an operation runs", func() {
		Expect(reg.Busy(app)).To(BeFalse())

		op, _, err := reg.Start(ctx, app, models.OperationBuild)
		Expect(err).ToNot(HaveOccurred())
		Expect(reg.Busy(app)).To(BeTrue())

		reg.Finish(app, op.ID, nil)
		Expect(reg.Busy(app)).To(BeFalse())
	})

	It("forgets a deleted application", func() {
		_, _, err := reg.Start(ctx, app, models.OperationBuild)
		Expect(err).ToNot(HaveOccurred())

		reg.Delete(app)
		_, ok := reg.Get(app)
		Expect(ok).To(BeFalse())

		_, _, err = reg.Start(ctx, app, models.OperationBuild)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("StateOf", func() {
	var app models.App

	BeforeEach(func() {
		app = models.App{
			Meta:          models.NewAppRef("state-app", "state-ns"),
			Status:        models.ApplicationRunning,
			StatusMessage: "1/1",
			StagingStatus: models.ApplicationStagingDone,
			BuildStatus:   models.AppBuildStatusDeployed,
			StageID:       "old-stage",
		}
	})

	AfterEach(func() {
		application.Operations.Delete(app.Meta)
	})

	start := func() models.AppOperation {
		op, _, err := application.Operations.Start(
			context.Background(), app.Meta, models.OperationDeploy,
		)
		Expect(err).ToNot(HaveOccurred())
		return op
	}

	It("reports the App CR alone when there is no operation", func() {
		state := application.StateOf(&app)

		Expect(state.Status).To(Equal(models.ApplicationStatus(
			models.ApplicationRunning,
		)))
		Expect(state.StageID).To(Equal("old-stage"))
		Expect(state.BuildStatus).To(Equal(models.AppBuildStatusDeployed))
		Expect(state.Operation).To(BeNil())
	})

	It("reports a staging operation as staging, with its stage", func() {
		op := start()
		application.Operations.Update(app.Meta, op.ID,
			func(o *models.AppOperation) {
				o.Phase = models.OperationStaging
				o.StageID = "new-stage"
			})

		state := application.StateOf(&app)

		Expect(state.Status).To(Equal(models.ApplicationStatus(
			models.ApplicationStaging,
		)))
		Expect(state.StageID).To(Equal("new-stage"))
		Expect(state.Operation.ID).To(Equal(op.ID))
	})

	It("reports a deploying operation as deploying", func() {
		op := start()
		application.Operations.Update(app.Meta, op.ID,
			func(o *models.AppOperation) {
				o.Phase = models.OperationDeploying
			})

		Expect(application.StateOf(&app).Status).To(Equal(
			models.ApplicationStatus(models.ApplicationDeploying),
		))
	})

	It("keeps the derived status once the operation is done", func() {
		op := start()
		application.Operations.Finish(app.Meta, op.ID, errors.New("boom"))

		state := application.StateOf(&app)

		Expect(state.Status).To(Equal(models.ApplicationStatus(
			models.ApplicationRunning,
		)))
		Expect(state.StageID).To(Equal("old-stage"))
		Expect(state.Operation.Phase).To(Equal(models.OperationFailed))
		Expect(state.Operation.Error).To(Equal("boom"))
	})

	It("attaches the operation to a looked-up app", func() {
		op := start()

		application.AttachOperation(&app)

		Expect(app.Operation).ToNot(BeNil())
		Expect(app.Operation.ID).To(Equal(op.ID))
	})
})
