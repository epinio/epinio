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
	"slices"
	"sync"
	"time"

	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// ErrOperationInProgress is returned by Start when the application already
// has an operation that has not finished. Handlers map it to a 409.
var ErrOperationInProgress = errors.New("an operation is already in progress")

// operationTTL is how long a finished operation stays readable, so a
// client polling GET .../state still sees the outcome and any error.
const operationTTL = 15 * time.Minute

// operationEntry pairs an operation with the time its record expires and
// the cancel function of the context its runner works under. expires is
// zero and cancel is set while the operation is still running.
type operationEntry struct {
	op      models.AppOperation
	expires time.Time
	cancel  context.CancelFunc
}

// OperationRegistry holds the latest operation per application, in memory.
// It is keyed by namespace/name, so it holds at most one entry per app.
// The App CR stays the durable record; see decisions 2026-11-04.
type OperationRegistry struct {
	mu      sync.Mutex
	entries map[string]*operationEntry
	ttl     time.Duration
	now     func() time.Time // replaceable in tests
	newID   func() (string, error)
}

// NewOperationRegistry returns an empty registry. newID generates
// operation IDs (randstr.Hex16 in production).
func NewOperationRegistry(
	ttl time.Duration,
	now func() time.Time,
	newID func() (string, error),
) *OperationRegistry {
	return &OperationRegistry{
		entries: map[string]*operationEntry{},
		ttl:     ttl,
		now:     now,
		newID:   newID,
	}
}

var Operations *OperationRegistry = NewOperationRegistry(
	operationTTL,
	time.Now,
	randstr.Hex16,
)

func operationKey(app models.AppRef) string {
	return app.Namespace + "/" + app.Name
}

// Start records a new pending operation for app and returns it, with a
// context derived from parent that the runner must do its work under.
// Finish and Delete cancel that context, so deleting the app stops a
// runner that is still staging or deploying.
func (r *OperationRegistry) Start(
	parent context.Context,
	app models.AppRef,
	kind models.OperationKind,
) (models.AppOperation, context.Context, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := operationKey(app)

	// A running entry never expires (expires is zero until Finish), so
	// Done() alone decides. A finished entry is simply replaced.
	if entry, ok := r.entries[key]; ok && !entry.op.Done() {
		return models.AppOperation{}, nil, ErrOperationInProgress
	}

	id, err := r.newID()
	if err != nil {
		return models.AppOperation{}, nil, fmt.Errorf(
			"generating operation id: %w", err,
		)
	}

	op := models.AppOperation{
		ID:        id,
		App:       app,
		Kind:      kind,
		Phase:     models.OperationPending,
		StartedAt: r.now().UTC().Format(time.RFC3339),
	}
	ctx, cancel := context.WithCancel(parent)
	r.entries[key] = &operationEntry{op: op, cancel: cancel}

	// op is a value and has no slices yet, so returning it is a copy.
	return op, ctx, nil
}

// Update applies mut to app's operation, but only while id is still the
// app's current operation. A late update from a superseded runner is
// dropped rather than overwriting a newer operation.
func (r *OperationRegistry) Update(
	app models.AppRef,
	id string,
	mut func(*models.AppOperation),
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[operationKey(app)]
	if !ok || entry.op.ID != id {
		return
	}

	mut(&entry.op)
}

// Finish marks app's operation as succeeded (err == nil) or failed, sets
// FinishedAt, and starts the expiry clock. Same id rule as Update.
func (r *OperationRegistry) Finish(app models.AppRef, id string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[operationKey(app)]
	if !ok || entry.op.ID != id {
		return
	}

	if err == nil {
		entry.op.Phase = models.OperationSucceeded
	} else {
		entry.op.Phase = models.OperationFailed
		entry.op.Error = err.Error()
	}

	now := r.now()
	entry.op.FinishedAt = now.UTC().Format(time.RFC3339)
	entry.expires = now.Add(r.ttl)

	// The runner is done; release its context.
	entry.cancel()
	entry.cancel = nil
}

// Get returns a copy of app's latest operation, if one is recorded and
// not expired. Expired entries are evicted here, so no sweeper is needed.
func (r *OperationRegistry) Get(
	app models.AppRef,
) (models.AppOperation, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[operationKey(app)]
	if !ok {
		return models.AppOperation{}, false
	}

	if !entry.expires.IsZero() && entry.expires.Before(r.now()) {
		delete(r.entries, operationKey(app))
		return models.AppOperation{}, false
	}

	// The struct copy still shares the slices' backing arrays with the
	// stored entry, so clone them before the caller can read them.
	op := entry.op
	op.Routes = slices.Clone(entry.op.Routes)
	op.Warnings = slices.Clone(entry.op.Warnings)

	return op, true
}

// Busy reports whether app has an operation that has not finished.
// Handlers that change the App CR before starting an operation check it
// first, so a busy app is rejected before anything is changed. It is
// advisory: Start remains the authoritative check.
func (r *OperationRegistry) Busy(app models.AppRef) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[operationKey(app)]
	return ok && !entry.op.Done()
}

// Delete drops app's record and cancels a runner that is still working.
// Called when the application is deleted, so a recreated app with the
// same name neither inherits a stale operation nor gets deployed to by
// the old runner.
func (r *OperationRegistry) Delete(app models.AppRef) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := operationKey(app)
	if entry, ok := r.entries[key]; ok && entry.cancel != nil {
		entry.cancel()
	}
	delete(r.entries, key)
}

// AttachOperation sets app.Operation to the app's latest operation, when
// the registry still holds one.
func AttachOperation(app *models.App) {
	if op, ok := Operations.Get(app.Meta); ok {
		app.Operation = &op
	}
}

// StateOf builds the app's lifecycle state from a looked-up app and the
// registry. A running operation's phase overrides the derived status: the
// App CR and the workload only show a deploy once Helm has acted, and a
// build only once its Job exists.
func StateOf(app *models.App) models.AppState {
	state := models.AppState{
		App:           app.Meta,
		Status:        app.Status,
		StatusMessage: app.StatusMessage,
		StagingStatus: app.StagingStatus,
		BuildStatus:   app.BuildStatus,
		StageID:       app.StageID,
	}

	op, ok := Operations.Get(app.Meta)
	if !ok {
		return state
	}
	state.Operation = &op

	switch {
	case op.Done():
	case op.Phase == models.OperationStaging:
		state.Status = models.ApplicationStaging
	case op.Phase == models.OperationDeploying:
		state.Status = models.ApplicationDeploying
	}
	if op.StageID != "" && !op.Done() {
		state.StageID = op.StageID
	}

	return state
}
