package middleware

import (
	"encoding/hex"
	"time"

	"github.com/cespare/xxhash/v2"
)

// ETagGenerator generates ETags from content.
type ETagGenerator struct{}

// NewETagGenerator creates a new ETag generator.
func NewETagGenerator() *ETagGenerator {
	return &ETagGenerator{}
}

// Generate creates a strong ETag from the provided content.
func (g *ETagGenerator) Generate(content []byte) string {
	hash := xxhash.Sum64(content)

	return hex.EncodeToString(uint64ToBytes(hash))
}

// GenerateFromString creates a strong ETag from a string.
func (g *ETagGenerator) GenerateFromString(content string) string {
	return g.Generate([]byte(content))
}

// GenerateWithTimestamp creates an ETag combining content hash and timestamp.
func (g *ETagGenerator) GenerateWithTimestamp(content []byte, timestamp time.Time) string {
	timestampBytes := uint64ToBytes(uint64(timestamp.UnixNano()))
	combined := append(content, timestampBytes...)

	return g.Generate(combined)
}

// GenerateWeak creates a weak ETag from the provided content.
func (g *ETagGenerator) GenerateWeak(content []byte) string {
	return "W/" + g.Generate(content)
}

func uint64ToBytes(val uint64) []byte {
	buf := make([]byte, 8)
	for i := 0; i < 8; i++ {
		buf[i] = byte(val >> (56 - uint(i)*8))
	}

	return buf
}
