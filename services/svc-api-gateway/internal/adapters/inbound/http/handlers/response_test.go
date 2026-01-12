package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ResponseSuite struct {
	suite.Suite
}

func TestResponseSuite(t *testing.T) {
	suite.Run(t, new(ResponseSuite))
}

func (s *ResponseSuite) TestNewMeta() {
	s.T().Parallel()

	cases := []struct {
		name            string
		requestID       string
		traceparent     string
		expectedTraceID string
	}{
		{
			name:            "with request ID and traceparent",
			requestID:       "550e8400-e29b-41d4-a716-446655440000",
			traceparent:     "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
			expectedTraceID: "0af7651916cd43dd8448eb211c80319c",
		},
		{
			name:            "with request ID only",
			requestID:       "550e8400-e29b-41d4-a716-446655440000",
			traceparent:     "",
			expectedTraceID: "",
		},
		{
			name:            "with invalid traceparent",
			requestID:       "550e8400-e29b-41d4-a716-446655440000",
			traceparent:     "invalid",
			expectedTraceID: "",
		},
		{
			name:            "with short traceparent",
			requestID:       "550e8400-e29b-41d4-a716-446655440000",
			traceparent:     "00-abc",
			expectedTraceID: "",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
			if tc.requestID != "" {
				req.Header.Set("Request-Id", tc.requestID)
			}
			if tc.traceparent != "" {
				req.Header.Set("traceparent", tc.traceparent)
			}

			meta := NewMeta(req)

			s.Require().Equal("v1", meta.APIVersion)
			s.Require().Equal(tc.expectedTraceID, meta.TraceID)
		})
	}
}

func (s *ResponseSuite) TestExtractTraceID() {
	s.T().Parallel()

	cases := []struct {
		name        string
		traceparent string
		expected    string
	}{
		{
			name:        "valid traceparent",
			traceparent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
			expected:    "0af7651916cd43dd8448eb211c80319c",
		},
		{
			name:        "empty traceparent",
			traceparent: "",
			expected:    "",
		},
		{
			name:        "short traceparent",
			traceparent: "00-abc",
			expected:    "",
		},
		{
			name:        "exactly minimum length",
			traceparent: "00-12345678901234567890123456789012-1234567890123456-01",
			expected:    "12345678901234567890123456789012",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
			if tc.traceparent != "" {
				req.Header.Set("traceparent", tc.traceparent)
			}

			result := extractTraceID(req)

			s.Require().Equal(tc.expected, result)
		})
	}
}

func (s *ResponseSuite) TestEnvelopedResponse_Structure() {
	s.T().Parallel()

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

	response := EnvelopedResponse{
		Data: map[string]string{"name": "Test Device"},
		Meta: NewMeta(req),
	}

	s.Require().NotNil(response.Data)
	s.Require().Equal("v1", response.Meta.APIVersion)
	s.Require().Equal("0af7651916cd43dd8448eb211c80319c", response.Meta.TraceID)
	s.Require().Nil(response.Pagination)
}

func (s *ResponseSuite) TestEnvelopedResponse_WithPagination() {
	s.T().Parallel()

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Request-Id", "550e8400-e29b-41d4-a716-446655440000")
	req.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

	hasNext := true
	hasPrevious := false
	pagination := &paginationData{
		Page:        1,
		Size:        20,
		TotalItems:  100,
		TotalPages:  5,
		HasNext:     &hasNext,
		HasPrevious: &hasPrevious,
	}

	response := EnvelopedResponse{
		Data:       []string{"device1", "device2"},
		Meta:       NewMeta(req),
		Pagination: pagination,
	}

	s.Require().NotNil(response.Data)
	s.Require().Equal("v1", response.Meta.APIVersion)
	s.Require().Equal("0af7651916cd43dd8448eb211c80319c", response.Meta.TraceID)
	s.Require().NotNil(response.Pagination)
	s.Require().Equal(uint(1), response.Pagination.Page)
	s.Require().Equal(uint(20), response.Pagination.Size)
	s.Require().Equal(uint(100), response.Pagination.TotalItems)
	s.Require().Equal(uint(5), response.Pagination.TotalPages)
	s.Require().True(*response.Pagination.HasNext)
	s.Require().False(*response.Pagination.HasPrevious)
}
