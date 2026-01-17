// Package idempotency provides utilities for implementing idempotent HTTP requests.
// It includes key validation, context management, and cache key generation.
package idempotency

import (
	"context"
)

const (
	// ContextKeyIdempotency is the context key for the idempotency key.
	ContextKeyIdempotency contextKey = "idempotencyKey"
)

type (
	contextKey string
)

// FromContext retrieves the idempotency key from context.
func FromContext(ctx context.Context) (string, bool) {
	key, ok := ctx.Value(ContextKeyIdempotency).(string)

	return key, ok && key != ""
}

// WithKey returns a new context with the idempotency key.
func WithKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, ContextKeyIdempotency, key)
}
