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

// Package requestctx provides access to special fields in the http request's context
package requestctx

import (
	"context"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/auth"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// IDKey is the unique key to lookup the request ID from the request's context
type IDKey struct{}

// UserKey is the unique key to lookup the username from the request's context
type UserKey struct{}

// WithUser adds the User to the context
func WithUser(ctx context.Context, val auth.User) context.Context {
	return context.WithValue(ctx, UserKey{}, val)
}

// User returns the User from the context
func User(ctx context.Context) auth.User {
	user, ok := ctx.Value(UserKey{}).(auth.User)
	if !ok {
		return auth.User{}
	}
	return user
}

// WithID adds the request ID to the context
func WithID(ctx context.Context, val string) context.Context {
	return context.WithValue(ctx, IDKey{}, val)
}

// ID returns the request ID from the context
func ID(ctx context.Context) string {
	id, ok := ctx.Value(IDKey{}).(string)
	if !ok {
		return ""
	}
	return id
}

// Logger returns the centralized logger enriched with request context when
// available. When the context carries a valid OpenTelemetry span, trace_id
// and span_id are added so standard log output can be correlated with traces.
// A context.Context field is also attached for the otelzap bridge (used as
// the emit context, not printed when TeeCore's console stripper is active).
func Logger(ctx context.Context) *zap.SugaredLogger {
	if helpers.Logger == nil {
		return zap.NewNop().Sugar()
	}

	fields := make([]zap.Field, 0, 4)
	if requestID := ID(ctx); requestID != "" {
		fields = append(fields, zap.String("requestId", requestID))
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		fields = append(fields,
			zap.String("trace_id", sc.TraceID().String()),
			zap.String("span_id", sc.SpanID().String()),
		)
	}
	// otelzap treats a context.Context field as the emit context and skips it
	// as an attribute. helpers.TeeCore strips it from the console core too.
	if len(fields) > 0 {
		fields = append(fields, zap.Any("context", ctx))
	}

	if len(fields) == 0 {
		return helpers.Logger
	}
	return helpers.Logger.Desugar().With(fields...).Sugar()
}
