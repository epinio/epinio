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

package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"

	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The registry is package-global, so every spec uses its own app name.
var operationSpecApps atomic.Int64

func uniqueOperationApp() models.AppRef {
	n := operationSpecApps.Add(1)
	return models.NewAppRef(fmt.Sprintf("op-app-%d", n), "op-ns")
}

// operationRequest builds a gin context whose request carries ctx.
func operationRequest(
	ctx context.Context,
) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil).
		WithContext(ctx)
	return c, w
}

// eventuallyOperation polls the registry until app's operation is done.
func eventuallyOperation(app models.AppRef) models.AppOperation {
	var op models.AppOperation
	Eventually(func() bool {
		var ok bool
		op, ok = application.Operations.Get(app)
		return ok && op.Done()
	}).Should(BeTrue())
	return op
}

var _ = Describe("Operation runner", func() {
	var app models.AppRef

	BeforeEach(func() {
		app = uniqueOperationApp()
		// The server binds this flag with default 1; unset, the operation
		// timeout is 0 and every operation times out at once.
		viper.Set("timeout-multiplier", 1)
	})

	AfterEach(func() {
		application.Operations.Delete(app)
	})

	It("answers 202 with a pending operation and a Location", func() {
		release := make(chan struct{})
		defer close(release)

		c, w := operationRequest(context.Background())
		apiErr := startOperation(
			c,
			app,
			models.OperationBuild,
			func(ctx context.Context, _ operationReporter) error {
				<-release
				return nil
			},
		)
		Expect(apiErr).To(BeNil())

		Expect(w.Code).To(Equal(http.StatusAccepted))
		Expect(w.Header().Get("Location")).To(Equal(
			"/api/v1/namespaces/op-ns/applications/" + app.Name + "/state",
		))

		var op models.AppOperation
		Expect(json.Unmarshal(w.Body.Bytes(), &op)).To(Succeed())
		Expect(op.ID).ToNot(BeEmpty())
		Expect(op.Kind).To(Equal(models.OperationBuild))
		Expect(op.Phase).To(Equal(models.OperationPending))
	})

	It("answers 409 while the app has a running operation", func() {
		release := make(chan struct{})
		defer close(release)

		c, _ := operationRequest(context.Background())
		Expect(startOperation(
			c,
			app,
			models.OperationBuild,
			func(ctx context.Context, _ operationReporter) error {
				<-release
				return nil
			},
		)).To(BeNil())

		var secondRan atomic.Bool
		c2, _ := operationRequest(context.Background())
		apiErr := startOperation(
			c2,
			app,
			models.OperationDeploy,
			func(ctx context.Context, _ operationReporter) error {
				secondRan.Store(true)
				return nil
			},
		)

		Expect(apiErr).ToNot(BeNil())
		Expect(apiErr.FirstStatus()).To(Equal(http.StatusConflict))
		Consistently(secondRan.Load).Should(BeFalse())
	})

	It("records success and the reported progress", func() {
		c, _ := operationRequest(context.Background())
		Expect(startOperation(
			c,
			app,
			models.OperationDeploy,
			func(ctx context.Context, rep operationReporter) error {
				rep.Phase(models.OperationStaging)
				rep.Staged("s1", "registry/img:s1")
				rep.Phase(models.OperationDeploying)
				rep.Deployed([]string{"app.example.com"}, nil)
				return nil
			},
		)).To(BeNil())

		op := eventuallyOperation(app)
		Expect(op.Phase).To(Equal(models.OperationSucceeded))
		Expect(op.StageID).To(Equal("s1"))
		Expect(op.ImageURL).To(Equal("registry/img:s1"))
		Expect(op.Routes).To(Equal([]string{"app.example.com"}))
		Expect(op.FinishedAt).ToNot(BeEmpty())
	})

	It("records a failure with the work's error", func() {
		c, _ := operationRequest(context.Background())
		Expect(startOperation(
			c,
			app,
			models.OperationRestart,
			func(ctx context.Context, _ operationReporter) error {
				return errors.New("helm upgrade failed")
			},
		)).To(BeNil())

		op := eventuallyOperation(app)
		Expect(op.Phase).To(Equal(models.OperationFailed))
		Expect(op.Error).To(Equal("helm upgrade failed"))
	})

	It("fails the operation when work panics", func() {
		c, _ := operationRequest(context.Background())
		Expect(startOperation(
			c,
			app,
			models.OperationBuild,
			func(ctx context.Context, _ operationReporter) error {
				panic("boom")
			},
		)).To(BeNil())

		op := eventuallyOperation(app)
		Expect(op.Phase).To(Equal(models.OperationFailed))
		Expect(op.Error).To(ContainSubstring("boom"))
	})

	It("cancels work when the app's operation is deleted", func() {
		canceled := make(chan struct{})

		c, _ := operationRequest(context.Background())
		Expect(startOperation(
			c,
			app,
			models.OperationDeploy,
			func(ctx context.Context, _ operationReporter) error {
				<-ctx.Done()
				close(canceled)
				return ctx.Err()
			},
		)).To(BeNil())

		application.Operations.Delete(app)
		Eventually(canceled).Should(BeClosed())
	})

	It("keeps the user and outlives the request context", func() {
		reqCtx, endRequest := context.WithCancel(
			requestctx.WithUser(
				context.Background(),
				auth.User{Username: "alice"},
			),
		)
		reqCtx = requestctx.WithID(reqCtx, "req-1")

		type seen struct {
			user string
			id   string
			err  error
		}
		got := make(chan seen, 1)
		proceed := make(chan struct{})

		c, _ := operationRequest(reqCtx)
		Expect(startOperation(
			c,
			app,
			models.OperationBuild,
			func(ctx context.Context, _ operationReporter) error {
				<-proceed
				got <- seen{
					user: requestctx.User(ctx).Username,
					id:   requestctx.ID(ctx),
					err:  ctx.Err(),
				}
				return nil
			},
		)).To(BeNil())

		endRequest()
		close(proceed)

		var s seen
		Eventually(got).Should(Receive(&s))
		Expect(s.user).To(Equal("alice"))
		Expect(s.id).To(Equal("req-1"))
		Expect(s.err).ToNot(HaveOccurred())
	})
})

var _ = Describe("apiErrorAsError", func() {
	It("returns nil for no error", func() {
		Expect(apiErrorAsError(nil)).To(Succeed())
	})

	It("keeps the title and details of the first error", func() {
		err := apiErrorAsError(
			apierror.NewInternalError("Failed to stage", "job failed"),
		)
		Expect(err).To(MatchError("Failed to stage: job failed"))
	})

	It("keeps a bare title", func() {
		err := apiErrorAsError(apierror.NewBadRequestError("bad input"))
		Expect(err).To(MatchError("bad input"))
	})
})

var _ = Describe("RedeployIfRunning", func() {
	var apps []models.AppRef

	deployedApp := func(status models.ApplicationStatus) models.App {
		ref := uniqueOperationApp()
		apps = append(apps, ref)
		return models.App{
			Meta:     ref,
			Workload: &models.AppDeployment{},
			Status:   status,
		}
	}

	AfterEach(func() {
		for _, app := range apps {
			application.Operations.Delete(app)
		}
		apps = nil
	})

	It("reports a deployed app that is not running as skipped", func() {
		app := deployedApp(models.ApplicationError)

		result := RedeployIfRunning(
			context.Background(),
			nil,
			[]models.App{app},
			false,
		)

		Expect(result.Operations).To(BeEmpty())
		Expect(result.Skipped).To(HaveLen(1))
		Expect(result.Skipped[0].App).To(Equal(app.Meta))
		Expect(result.Skipped[0].Reason).To(ContainSubstring("error"))
	})

	It("ignores an app that was never deployed", func() {
		result := RedeployIfRunning(
			context.Background(),
			nil,
			[]models.App{{Meta: uniqueOperationApp()}},
			false,
		)

		Expect(result.Operations).To(BeEmpty())
		Expect(result.Skipped).To(BeEmpty())
	})

	It("reports a running app that is busy as skipped", func() {
		app := deployedApp(models.ApplicationRunning)
		_, _, err := application.Operations.Start(
			context.Background(),
			app.Meta,
			models.OperationBuild,
		)
		Expect(err).ToNot(HaveOccurred())

		result := RedeployIfRunning(
			context.Background(),
			nil,
			[]models.App{app},
			false,
		)

		Expect(result.Operations).To(BeEmpty())
		Expect(result.Skipped).To(HaveLen(1))
		Expect(result.Skipped[0].Reason).To(
			ContainSubstring("another operation is running"),
		)
	})

	It("handles an app listed twice once", func() {
		app := deployedApp(models.ApplicationError)

		result := RedeployIfRunning(
			context.Background(),
			nil,
			[]models.App{app, app},
			false,
		)

		Expect(result.Skipped).To(HaveLen(1))
	})
})

var _ = Describe("Redeploy responses", func() {
	op := models.AppOperation{ID: "op1"}
	skip := models.SkippedApp{Reason: "busy"}

	It("is 202 only when an operation started", func() {
		Expect(RedeployStatus(models.OperationsResponse{})).
			To(Equal(http.StatusOK))
		Expect(RedeployStatus(models.OperationsResponse{
			Skipped: []models.SkippedApp{skip},
		})).To(Equal(http.StatusOK))
		Expect(RedeployStatus(models.OperationsResponse{
			Operations: []models.AppOperation{op},
		})).To(Equal(http.StatusAccepted))
	})

	It("keeps the plain 200 when nothing was redeployed", func() {
		c, w := operationRequest(context.Background())
		RespondRedeploys(c, models.OperationsResponse{})

		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Body.String()).ToNot(ContainSubstring("skipped"))
	})

	It("reports skipped apps even when nothing started", func() {
		c, w := operationRequest(context.Background())
		RespondRedeploys(c, models.OperationsResponse{
			Skipped: []models.SkippedApp{skip},
		})

		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Body.String()).To(ContainSubstring(`"skipped"`))
	})

	It("embeds the list in the bind response body", func() {
		body, err := json.Marshal(models.BindResponse{
			WasBound: []string{"cfg"},
			OperationsResponse: models.OperationsResponse{
				Operations: []models.AppOperation{op},
			},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(string(body)).To(ContainSubstring(`"wasbound":["cfg"]`))
		Expect(string(body)).To(ContainSubstring(`"operations":[`))
	})
})
