package model

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidCursor = errors.New("invalid cursor")
)

type (
	// CursorDirection indicates the pagination direction.
	CursorDirection string

	// Cursor represents a pagination cursor for keyset pagination.
	Cursor struct {
		Field     string          `json:"f"`
		Value     any             `json:"v"`
		ID        string          `json:"id"`
		Direction CursorDirection `json:"d"`
	}
)

const (
	CursorDirectionNext CursorDirection = "next"
	CursorDirectionPrev CursorDirection = "prev"
)

// EncodeCursor serializes a cursor to a URL-safe base64 string.
func EncodeCursor(c Cursor) (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal cursor: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

// DecodeCursor deserializes a cursor from a base64 string.
func DecodeCursor(encoded string) (Cursor, error) {
	if encoded == "" {
		return Cursor{}, ErrInvalidCursor
	}

	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return Cursor{}, fmt.Errorf("%w: %v", ErrInvalidCursor, err)
	}

	var c Cursor
	if err := json.Unmarshal(data, &c); err != nil {
		return Cursor{}, fmt.Errorf("%w: %v", ErrInvalidCursor, err)
	}

	return c, nil
}

// NewCursorFromDevice creates a cursor from a device for the given sort field.
func NewCursorFromDevice(device *Device, sortField string, direction CursorDirection) Cursor {
	var value any

	switch sortField {
	case "created_at", "-created_at", "createdAt", "-createdAt":
		value = device.CreatedAt.Format(time.RFC3339Nano)
	case "updated_at", "-updated_at", "updatedAt", "-updatedAt":
		value = device.UpdatedAt.Format(time.RFC3339Nano)
	case "name", "-name":
		value = device.Name
	case "brand", "-brand":
		value = device.Brand
	case "state", "-state":
		value = string(device.State)
	default:
		value = device.CreatedAt.Format(time.RFC3339Nano)
	}

	return Cursor{
		Field:     sortField,
		Value:     value,
		ID:        device.ID.String(),
		Direction: direction,
	}
}

// ParseCursorValue extracts the typed value from a cursor for SQL comparison.
func (c *Cursor) ParseCursorValue() (any, error) {
	switch c.Field {
	case "created_at", "-created_at", "createdAt", "-createdAt",
		"updated_at", "-updated_at", "updatedAt", "-updatedAt":
		if strVal, ok := c.Value.(string); ok {
			return time.Parse(time.RFC3339Nano, strVal)
		}

		return nil, fmt.Errorf("%w: expected time string", ErrInvalidCursor)
	default:
		return c.Value, nil
	}
}
