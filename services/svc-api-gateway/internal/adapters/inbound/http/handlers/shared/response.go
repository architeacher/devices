package shared

import (
	"net/http"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
)

const (
	apiVersion = "v1"

	// W3C Trace Context traceparent header format components.
	// Format: {version}-{trace-id}-{parent-id}-{trace-flags}
	// Example: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
	traceparentVersionLength   = 2  // "00"
	traceparentSeparatorLength = 1  // "-"
	traceparentTraceIDLength   = 32 // hex characters
	traceparentParentIDLength  = 16 // hex characters
	traceparentFlagsLength     = 2  // hex characters

	// Derived indices for parsing.
	traceparentTraceIDStart = traceparentVersionLength + traceparentSeparatorLength // 3
	traceparentTraceIDEnd   = traceparentTraceIDStart + traceparentTraceIDLength    // 35
	traceparentMinLength    = traceparentVersionLength + traceparentSeparatorLength +
		traceparentTraceIDLength + traceparentSeparatorLength +
		traceparentParentIDLength + traceparentSeparatorLength +
		traceparentFlagsLength // 55
)

type (
	// PaginationData contains pagination information for list responses.
	PaginationData struct {
		Page           uint    `json:"page"`
		Size           uint    `json:"size"`
		TotalItems     uint    `json:"totalItems"`
		TotalPages     uint    `json:"totalPages"`
		HasNext        *bool   `json:"hasNext,omitempty"`
		HasPrevious    *bool   `json:"hasPrevious,omitempty"`
		NextCursor     *string `json:"nextCursor,omitempty"`
		PreviousCursor *string `json:"previousCursor,omitempty"`
	}

	// ResponseMeta contains response metadata for tracing and API versioning.
	ResponseMeta struct {
		RequestID  string `json:"requestId"`
		TraceID    string `json:"traceId,omitempty"`
		APIVersion string `json:"apiVersion"`
	}

	// EnvelopedResponse wraps response data with metadata and optional pagination.
	EnvelopedResponse struct {
		Data       any             `json:"data"`
		Meta       ResponseMeta    `json:"meta"`
		Pagination *PaginationData `json:"pagination,omitempty"`
	}
)

// NewMeta creates response metadata from the request context.
func NewMeta(r *http.Request) ResponseMeta {
	return ResponseMeta{
		RequestID:  middleware.GetRequestID(r.Context()),
		TraceID:    ExtractTraceID(r),
		APIVersion: apiVersion,
	}
}

// ExtractTraceID extracts the trace ID from the traceparent header.
// Format: {version}-{trace-id}-{parent-id}-{trace-flags}
// Example: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
func ExtractTraceID(r *http.Request) string {
	traceparent := r.Header.Get("traceparent")
	if traceparent == "" {
		return ""
	}

	// Parse traceparent header (W3C Trace Context format).
	if len(traceparent) < traceparentMinLength {
		return ""
	}

	return traceparent[traceparentTraceIDStart:traceparentTraceIDEnd]
}
