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
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/duration"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

// operationTimeout bounds one operation: one full build plus one deploy.
// It replaces the separate client-side deadlines (the CLI used 10m for
// both together), so only the server decides when an operation is late.
func operationTimeout() time.Duration {
	return duration.ToAppBuilt() + duration.ToDeployment()
}

// operationWork is the body of an operation. It runs in the background
// under ctx, which is canceled on timeout or when the app is deleted, and
// reports progress through rep. A returned error fails the operation.
type operationWork func(ctx context.Context, rep operationReporter) error

// operationReporter lets work report progress without knowing about the
// registry. It carries the operation ID, so a superseded runner's
// reports are dropped by the registry's ID check.
type operationReporter struct {
	app models.AppRef
	id  string
}

// Phase moves the operation to p (staging, deploying).
func (r operationReporter) Phase(p models.OperationPhase) {
	application.Operations.Update(r.app, r.id, func(o *models.AppOperation) {
		o.Phase = p
	})
}

// Staged records the staging run and the image it will produce.
func (r operationReporter) Staged(stageID, imageURL string) {
	application.Operations.Update(r.app, r.id, func(o *models.AppOperation) {
		o.StageID = stageID
		o.ImageURL = imageURL
	})
}

// Deployed records what the deploy reported back.
func (r operationReporter) Deployed(routes, warnings []string) {
	application.Operations.Update(r.app, r.id, func(o *models.AppOperation) {
		o.Routes = routes
		o.Warnings = warnings
	})
}

// operationParent returns the context a background operation starts
// from. It must not be the request context, which ends with the
// response, but it keeps what the runner needs from the request: the
// user (authorizeOrigin checks it), the request ID and the trace span
// (so the runner's log lines correlate with the request).
func operationParent(reqCtx context.Context) context.Context {
	parent := requestctx.WithUser(
		context.Background(),
		requestctx.User(reqCtx),
	)
	parent = requestctx.WithID(parent, requestctx.ID(reqCtx))

	if sc := trace.SpanContextFromContext(reqCtx); sc.IsValid() {
		parent = trace.ContextWithSpanContext(parent, sc)
	}

	return parent
}

// launchOperation registers an operation of kind for app and runs work in
// the background. It is the part of startOperation that fan-out handlers
// (one request, many apps) share: they call it once per app and report
// application.ErrOperationInProgress as a skipped app.
func launchOperation(
	parent context.Context,
	app models.AppRef,
	kind models.OperationKind,
	work operationWork,
) (models.AppOperation, error) {
	op, opCtx, err := application.Operations.Start(parent, app, kind)
	if err != nil {
		return models.AppOperation{}, err
	}

	go runOperation(opCtx, op, work)

	return op, nil
}

// startOperation registers an operation of kind for app, runs work in the
// background, and answers the request with 202 and the operation. The
// handler validates its input first, so bad requests still get a fast
// 4xx and never start an operation.
func startOperation(
	c *gin.Context,
	app models.AppRef,
	kind models.OperationKind,
	work operationWork,
) apierror.APIErrors {
	parent := operationParent(c.Request.Context())

	op, err := launchOperation(parent, app, kind, work)
	if errors.Is(err, application.ErrOperationInProgress) {
		return operationInProgress(app)
	}
	if err != nil {
		return apierror.InternalError(err, "starting the operation")
	}

	c.Header("Location", appStatePath(app))
	c.JSON(http.StatusAccepted, op)

	return nil
}

// operationInProgress is the 409 for an app that is already busy.
func operationInProgress(app models.AppRef) apierror.APIErrors {
	return apierror.NewAPIError(
		fmt.Sprintf(
			"application '%s' already has an operation in progress",
			app.Name,
		),
		http.StatusConflict,
	)
}

// appStatePath is where a client polls an app's state.
func appStatePath(app models.AppRef) string {
	return fmt.Sprintf(
		"/api/v1/namespaces/%s/applications/%s/state",
		app.Namespace,
		app.Name,
	)
}

// runOperation executes work under the operation deadline and records the
// outcome. Finish is called exactly once, even when work panics.
func runOperation(
	ctx context.Context,
	op models.AppOperation,
	work operationWork,
) {
	timeout := operationTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log := requestctx.Logger(ctx).With(
		"component", "app-operation",
		"operation", op.ID,
		"kind", op.Kind,
		"namespace", op.App.Namespace,
		"app", op.App.Name,
	)

	// An unrecovered panic in a goroutine kills the whole server, so a
	// bug in work fails the operation instead.
	defer func() {
		if p := recover(); p != nil {
			log.Errorw("operation panicked",
				"panic", p,
				"stack", string(debug.Stack()),
			)
			application.Operations.Finish(
				op.App,
				op.ID,
				fmt.Errorf("operation panicked: %v", p),
			)
		}
	}()

	err := work(ctx, operationReporter{app: op.App, id: op.ID})

	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = fmt.Errorf("timed out after %s: %w", timeout, err)
	}

	if err != nil {
		log.Infow("operation failed", "error", err)
	} else {
		log.Infow("operation succeeded")
	}

	application.Operations.Finish(op.App, op.ID, err)
}

// apiErrorAsError turns the APIErrors that staging and deploy helpers
// return into a plain error for operationWork. It keeps the first
// error's title, as failAPI in deployments.go does today, and appends
// its details, which usually carry the underlying cause.
func apiErrorAsError(apiErr apierror.APIErrors) error {
	if apiErr == nil {
		return nil
	}

	errs := apiErr.Errors()
	if len(errs) == 0 || errs[0].Title == "" {
		return errors.New("request failed")
	}

	first := errs[0]
	if first.Details != "" {
		return fmt.Errorf("%s: %s", first.Title, first.Details)
	}

	return errors.New(first.Title)
}

// StartRedeploys starts one update operation per app that redeploys it,
// so a change saved on its App CR (environment, bindings, the content of
// a bound configuration) reaches the workload. restart forces new pods
// even when the deployment itself does not change, which a changed
// configuration needs to be remounted. An app that is already busy is
// skipped, not failed: its change is saved and its next deploy uses it.
// Every handler that changes what is set or bound on an app uses this, so
// none of them answers 409; explicit lifecycle requests (stage, deploy,
// restart, update, source) do.
func StartRedeploys(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	apps []models.AppRef,
	restart bool,
) models.OperationsResponse {
	parent := operationParent(ctx)
	username := requestctx.User(ctx).Username
	log := requestctx.Logger(ctx)

	result := models.OperationsResponse{}
	for _, app := range apps {
		op, err := launchOperation(parent, app, models.OperationUpdate,
			func(ctx context.Context, rep operationReporter) error {
				rep.Phase(models.OperationDeploying)
				return redeployApp(ctx, cluster, app, username, restart)
			})
		if err != nil {
			reason := err.Error()
			if errors.Is(err, application.ErrOperationInProgress) {
				reason = "another operation is running; " +
					"the change applies on its next deploy"
			}
			log.Infow("redeploy skipped", "app", app.Name, "reason", reason)
			result.Skipped = append(result.Skipped, models.SkippedApp{
				App:    app,
				Reason: reason,
			})
			continue
		}

		result.Operations = append(result.Operations, op)
	}

	return result
}

// RedeployIfRunning redeploys each app whose workload is running, so a
// change saved on its App CR reaches it. Every other deployed app is
// reported as skipped: its change applies on its next deploy. Apps that
// were never deployed have nothing to redeploy and are not reported.
// Handlers collect all affected apps first and call this once, so an app
// changed twice in one request is redeployed once, not skipped as busy.
func RedeployIfRunning(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	apps []models.App,
	restart bool,
) models.OperationsResponse {
	result := models.OperationsResponse{}
	running := []models.AppRef{}
	seen := map[string]bool{}

	for _, app := range apps {
		if app.Workload == nil || seen[app.Meta.Name] {
			continue
		}
		seen[app.Meta.Name] = true

		if app.Status != models.ApplicationRunning {
			result.Skipped = append(result.Skipped, models.SkippedApp{
				App: app.Meta,
				Reason: fmt.Sprintf(
					"application is %s; the change applies on its next deploy",
					app.Status,
				),
			})
			continue
		}

		running = append(running, app.Meta)
	}

	started := StartRedeploys(ctx, cluster, running, restart)
	result.Operations = started.Operations
	result.Skipped = append(result.Skipped, started.Skipped...)

	return result
}

// RedeployStatus is the status for a response that reports redeploys:
// 202 when it started any operation, else 200.
func RedeployStatus(result models.OperationsResponse) int {
	if result.Started() {
		return http.StatusAccepted
	}

	return http.StatusOK
}

// RespondRedeploys answers a request whose only result is its redeploys:
// 202 with the list when it started any or skipped an app, else the plain
// 200 the handler gave before redeploys became operations.
func RespondRedeploys(c *gin.Context, result models.OperationsResponse) {
	if !result.Started() && len(result.Skipped) == 0 {
		response.OK(c)
		return
	}

	c.JSON(RedeployStatus(result), result)
}

func redeployApp(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app models.AppRef,
	username string,
	restart bool,
) error {
	if restart {
		_, apiErr := deploy.DeployAppWithRestart(
			ctx, cluster, app, username, "",
		)
		return apiErrorAsError(apiErr)
	}

	_, apiErr := deploy.DeployApp(ctx, cluster, app, username, "")
	return apiErrorAsError(apiErr)
}
