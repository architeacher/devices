package idempotency

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromContext(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		setupContext  func(t *testing.T) context.Context
		expectedKey   string
		expectedFound bool
	}{
		{
			name: "key exists in context",
			setupContext: func(t *testing.T) context.Context {
				return WithKey(t.Context(), "test-key-12345678")
			},
			expectedKey:   "test-key-12345678",
			expectedFound: true,
		},
		{
			name: "key not in context",
			setupContext: func(t *testing.T) context.Context {
				return t.Context()
			},
			expectedKey:   "",
			expectedFound: false,
		},
		{
			name: "empty key in context",
			setupContext: func(t *testing.T) context.Context {
				return WithKey(t.Context(), "")
			},
			expectedKey:   "",
			expectedFound: false,
		},
		{
			name: "key with UUID format",
			setupContext: func(t *testing.T) context.Context {
				return WithKey(t.Context(), "550e8400-e29b-41d4-a716-446655440000")
			},
			expectedKey:   "550e8400-e29b-41d4-a716-446655440000",
			expectedFound: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := tc.setupContext(t)
			key, found := FromContext(ctx)

			require.Equal(t, tc.expectedFound, found)
			require.Equal(t, tc.expectedKey, key)
		})
	}
}

func TestWithKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		key  string
	}{
		{
			name: "standard key",
			key:  "my-idempotency-key-123",
		},
		{
			name: "UUID key",
			key:  "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name: "key with underscores",
			key:  "my_key_with_underscores",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := WithKey(t.Context(), tc.key)

			key, found := FromContext(ctx)

			require.True(t, found)
			require.Equal(t, tc.key, key)
		})
	}
}

func TestWithKey_Overwrites(t *testing.T) {
	t.Parallel()

	ctx := WithKey(t.Context(), "first-key-12345678")
	ctx = WithKey(ctx, "second-key-1234567")

	key, found := FromContext(ctx)

	require.True(t, found)
	require.Equal(t, "second-key-1234567", key)
}

func TestContextKey_DoesNotConflict(t *testing.T) {
	t.Parallel()

	type otherContextKey string
	otherKey := otherContextKey("idempotencyKey")

	ctx := context.WithValue(t.Context(), otherKey, "other-value")
	ctx = WithKey(ctx, "idempotency-value1")

	idempotencyVal, found := FromContext(ctx)
	require.True(t, found)
	require.Equal(t, "idempotency-value1", idempotencyVal)

	otherVal := ctx.Value(otherKey)
	require.Equal(t, "other-value", otherVal)
}
