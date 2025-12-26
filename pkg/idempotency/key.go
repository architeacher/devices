package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
)

const (
	MinKeyLength = 16
	MaxKeyLength = 128
	KeyPrefix    = "idempotency"
)

var (
	ErrKeyTooShort = errors.New("idempotency key must be at least 16 characters")
	ErrKeyTooLong  = errors.New("idempotency key must not exceed 128 characters")
	ErrKeyInvalid  = errors.New("idempotency key contains invalid characters")

	validKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)
)

// Validate checks if the idempotency key is valid.
func Validate(key string) error {
	if len(key) < MinKeyLength {
		return ErrKeyTooShort
	}

	if len(key) > MaxKeyLength {
		return ErrKeyTooLong
	}

	if !validKeyPattern.MatchString(key) {
		return ErrKeyInvalid
	}

	return nil
}

// BuildCacheKey constructs the cache key from a method, path, and idempotency key.
func BuildCacheKey(method, path, idempotencyKey string) string {
	combined := fmt.Sprintf("%s:%s:%s", method, path, idempotencyKey)
	hash := sha256.Sum256([]byte(combined))

	return fmt.Sprintf("%s:%s", KeyPrefix, hex.EncodeToString(hash[:]))
}
