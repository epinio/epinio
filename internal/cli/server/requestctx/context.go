// Package requestctx provides access to special fields in the http request's context
package requestctx

import "context"

// IDKey is the unique key to lookup the request ID from the request's context
type IDKey struct{}

// UserKey is the unique key to lookup the username from the request's context
type UserKey struct{}

// ContextWithUser adds the user name to the context
func ContextWithUser(ctx context.Context, val string) context.Context {
	return context.WithValue(ctx, UserKey{}, val)
}

// User returns the user name from the context
func User(ctx context.Context) string {
	val, ok := ctx.Value(UserKey{}).(string)
	if ok {
		return val
	}
	return ""
}

// ContextWithID adds the user name to the context
func ContextWithID(ctx context.Context, val string) context.Context {
	return context.WithValue(ctx, IDKey{}, val)
}

// ID returns the user name from the context
func ID(ctx context.Context) string {
	val, ok := ctx.Value(IDKey{}).(string)
	if ok {
		return val
	}
	return ""
}
