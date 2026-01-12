package handlers

import (
	"net/http"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
)

const (
	apiVersion = "v1"
)

type (
	// responseMeta contains response metadata for tracing and API versioning.
	responseMeta struct {
		RequestID  string `json:"requestId"`
		TraceID    string `json:"traceId,omitempty"`
		APIVersion string `json:"apiVersion"`
	}

	// EnvelopedResponse wraps response data with metadata and optional pagination.
	EnvelopedResponse struct {
		Data       any             `json:"data"`
		Meta       responseMeta    `json:"meta"`
		Pagination *paginationData `json:"pagination,omitempty"`
	}
)

// NewMeta creates response metadata from the request context.
func NewMeta(r *http.Request) responseMeta {
	return responseMeta{
		RequestID:  middleware.GetRequestID(r.Context()),
		TraceID:    extractTraceID(r),
		APIVersion: apiVersion,
	}
}

// extractTraceID extracts the trace ID from the traceparent header.
// Format: {version}-{trace-id}-{parent-id}-{trace-flags}
// Example: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
func extractTraceID(r *http.Request) string {
	traceparent := r.Header.Get("traceparent")
	if traceparent == "" {
		return ""
	}

	// Parse traceparent header (W3C Trace Context format).
	// Minimum length: 2 (version) + 1 (-) + 32 (trace-id) + 1 (-) + 16 (parent-id) + 1 (-) + 2 (flags) = 55.
	if len(traceparent) < 55 {
		return ""
	}

	// Extract trace-id (characters 3-34, after "00-").
	return traceparent[3:35]
}
