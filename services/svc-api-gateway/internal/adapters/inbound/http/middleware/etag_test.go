package middleware_test

import (
	"strings"
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
	"github.com/stretchr/testify/require"
)

func TestETagGenerator_Generate(t *testing.T) {
	t.Parallel()

	generator := middleware.NewETagGenerator()

	cases := []struct {
		name    string
		content []byte
	}{
		{
			name:    "empty content",
			content: []byte{},
		},
		{
			name:    "simple content",
			content: []byte("hello world"),
		},
		{
			name:    "json content",
			content: []byte(`{"id":"123","name":"Test Device"}`),
		},
		{
			name:    "binary content",
			content: []byte{0x00, 0x01, 0x02, 0x03, 0xFF},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			etag := generator.Generate(tc.content)

			require.NotEmpty(t, etag)
			require.Len(t, etag, 16)
			require.True(t, isHex(etag), "ETag should be hexadecimal")
		})
	}
}

func TestETagGenerator_Generate_Consistency(t *testing.T) {
	t.Parallel()

	generator := middleware.NewETagGenerator()
	content := []byte("consistent content")

	etag1 := generator.Generate(content)
	etag2 := generator.Generate(content)

	require.Equal(t, etag1, etag2, "Same content should produce same ETag")
}

func TestETagGenerator_Generate_Uniqueness(t *testing.T) {
	t.Parallel()

	generator := middleware.NewETagGenerator()
	content1 := []byte("content one")
	content2 := []byte("content two")

	etag1 := generator.Generate(content1)
	etag2 := generator.Generate(content2)

	require.NotEqual(t, etag1, etag2, "Different content should produce different ETags")
}

func TestETagGenerator_GenerateFromString(t *testing.T) {
	t.Parallel()

	generator := middleware.NewETagGenerator()
	content := "hello world"

	etag := generator.GenerateFromString(content)

	require.NotEmpty(t, etag)
	require.Len(t, etag, 16)

	etagFromBytes := generator.Generate([]byte(content))
	require.Equal(t, etag, etagFromBytes, "String and bytes should produce same ETag")
}

func TestETagGenerator_GenerateWithTimestamp(t *testing.T) {
	t.Parallel()

	generator := middleware.NewETagGenerator()
	content := []byte("test content")
	timestamp1 := time.Date(2026, 1, 13, 12, 0, 0, 0, time.UTC)
	timestamp2 := time.Date(2026, 1, 13, 12, 0, 1, 0, time.UTC)

	etag1 := generator.GenerateWithTimestamp(content, timestamp1)
	etag2 := generator.GenerateWithTimestamp(content, timestamp2)
	etag3 := generator.GenerateWithTimestamp(content, timestamp1)

	require.NotEmpty(t, etag1)
	require.Len(t, etag1, 16)
	require.NotEqual(t, etag1, etag2, "Different timestamps should produce different ETags")
	require.Equal(t, etag1, etag3, "Same timestamp should produce same ETag")
}

func TestETagGenerator_GenerateWeak(t *testing.T) {
	t.Parallel()

	generator := middleware.NewETagGenerator()
	content := []byte("test content")

	weakEtag := generator.GenerateWeak(content)
	strongEtag := generator.Generate(content)

	require.True(t, strings.HasPrefix(weakEtag, "W/"), "Weak ETag should have W/ prefix")
	require.Equal(t, "W/"+strongEtag, weakEtag, "Weak ETag should be W/ followed by strong ETag")
}

func isHex(s string) bool {
	for _, c := range s {
		isDigit := c >= '0' && c <= '9'
		isLowerHex := c >= 'a' && c <= 'f'
		isUpperHex := c >= 'A' && c <= 'F'

		if !isDigit && !isLowerHex && !isUpperHex {
			return false
		}
	}

	return true
}
