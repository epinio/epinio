// Package requestctx provides access to special fields in the http request's context
package requestctx

import (
	"context"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/auth"
	"github.com/go-logr/logr"
)

// IDKey is the unique key to lookup the request ID from the request's context
type IDKey struct{}

// UserKey is the unique key to lookup the username from the request's context
type UserKey struct{}

// LoggerKey is the unique key to lookup the logger from the request's context
type LoggerKey struct{}

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
