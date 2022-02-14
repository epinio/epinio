// Package requestctx provides access to special fields in the http request's context
package requestctx

import (
	"context"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/go-logr/logr"
)

// IDKey is the unique key to lookup the request ID from the request's context
type IDKey struct{}

// UserKey is the unique key to lookup the username from the request's context
type UserKey struct{}

// LoggerKey is the unique key to lookup the logger from the request's context
type LoggerKey struct{}

// WithUser adds the user name to the context
func WithUser(ctx context.Context, val string) context.Context {
	return context.WithValue(ctx, UserKey{}, val)
}

// User returns the user name from the context
func User(ctx context.Context) string {
	return extractString(ctx, UserKey{})
}

// WithID adds the request ID to the context
func WithID(ctx context.Context, val string) context.Context {
	return context.WithValue(ctx, IDKey{}, val)
}

// ID returns the request ID from the context
func ID(ctx context.Context) string {
	return extractString(ctx, IDKey{})
}

// WithLogger returns a copy of the context with the given logger
func WithLogger(ctx context.Context, log logr.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey{}, log)
}

// Logger returns the logger from the context
func Logger(ctx context.Context) logr.Logger {
	log, ok := ctx.Value(LoggerKey{}).(logr.Logger)
	if !ok {
		// this should not happen, but let's be cautious
		return tracelog.NewLogger().WithName("fallback")
	}
	return log
}

func extractString(ctx context.Context, key interface{}) string {
	val, ok := ctx.Value(key).(string)
	if ok {
		return val
	}
	return ""
}
