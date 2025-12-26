package idempotency

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		key         string
		expectedErr error
	}{
		{
			name:        "valid UUID format key",
			key:         "550e8400-e29b-41d4-a716-446655440000",
			expectedErr: nil,
		},
		{
			name:        "valid alphanumeric key",
			key:         "my-idempotency-key-12345",
			expectedErr: nil,
		},
		{
			name:        "valid key with underscores",
			key:         "my_idempotency_key_12345",
			expectedErr: nil,
		},
		{
			name:        "key too short",
			key:         "short",
			expectedErr: ErrKeyTooShort,
		},
		{
			name:        "key exactly minimum length",
			key:         "1234567890123456",
			expectedErr: nil,
		},
		{
			name:        "key too long",
			key:         strings.Repeat("a", MaxKeyLength+1),
			expectedErr: ErrKeyTooLong,
		},
		{
			name:        "key exactly maximum length",
			key:         strings.Repeat("a", MaxKeyLength),
			expectedErr: nil,
		},
		{
			name:        "key with invalid characters",
			key:         "invalid!key@12345",
			expectedErr: ErrKeyInvalid,
		},
		{
			name:        "key with spaces",
			key:         "key with spaces 123",
			expectedErr: ErrKeyInvalid,
		},
		{
			name:        "empty key",
			key:         "",
			expectedErr: ErrKeyTooShort,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := Validate(tc.key)

			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestBuildCacheKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		method         string
		path           string
		idempotencyKey string
	}{
		{
			name:           "POST request",
			method:         "POST",
			path:           "/v1/devices",
			idempotencyKey: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:           "PUT request",
			method:         "PUT",
			path:           "/v1/devices/123",
			idempotencyKey: "my-idempotency-key-12345",
		},
		{
			name:           "PATCH request",
			method:         "PATCH",
			path:           "/v1/devices/456",
			idempotencyKey: "another_key_with_underscores",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key := BuildCacheKey(tc.method, tc.path, tc.idempotencyKey)

			require.True(t, strings.HasPrefix(key, KeyPrefix+":"))
			require.Len(t, key, len(KeyPrefix)+1+64) // prefix + ":" + sha256 hex (64 chars)
		})
	}
}

func TestBuildCacheKey_Deterministic(t *testing.T) {
	t.Parallel()

	method := "POST"
	path := "/v1/devices"
	idempotencyKey := "test-key-12345678"

	key1 := BuildCacheKey(method, path, idempotencyKey)
	key2 := BuildCacheKey(method, path, idempotencyKey)

	require.Equal(t, key1, key2, "cache keys should be deterministic")
}

func TestBuildCacheKey_UniqueForDifferentInputs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		method1 string
		path1   string
		key1    string
		method2 string
		path2   string
		key2    string
	}{
		{
			name:    "different methods",
			method1: "POST",
			path1:   "/v1/devices",
			key1:    "same-key-12345678",
			method2: "PUT",
			path2:   "/v1/devices",
			key2:    "same-key-12345678",
		},
		{
			name:    "different paths",
			method1: "POST",
			path1:   "/v1/devices",
			key1:    "same-key-12345678",
			method2: "POST",
			path2:   "/v1/users",
			key2:    "same-key-12345678",
		},
		{
			name:    "different idempotency keys",
			method1: "POST",
			path1:   "/v1/devices",
			key1:    "key-one-12345678",
			method2: "POST",
			path2:   "/v1/devices",
			key2:    "key-two-12345678",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cacheKey1 := BuildCacheKey(tc.method1, tc.path1, tc.key1)
			cacheKey2 := BuildCacheKey(tc.method2, tc.path2, tc.key2)

			require.NotEqual(t, cacheKey1, cacheKey2, "cache keys should be unique for different inputs")
		})
	}
}
