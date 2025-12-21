# Features

This document describes the features and API best practices implemented in the Devices API.

## TL;DR

| Feature | Status | Description |
|---------|--------|-------------|
| API Versioning | ✅ | URL path (`/v1/`) + header (`API-Version`) |
| Authentication | ✅ | PASETO v4 tokens with claims extraction |
| Authorization | ✅ | Basic Auth for admin endpoints |
| Request Validation | ✅ | OpenAPI 3.0.3 schema-based validation |
| Error Handling | ✅ | Structured responses with gRPC error mapping |
| Pagination | ✅ | `page`/`size` params with metadata |
| Filtering & Sorting | ✅ | Brand, state filters; multi-field sorting |
| Field Projection | ✅ | Sparse fieldsets via `fields` parameter |
| HATEOAS | ✅ | Self links in responses |
| Content Negotiation | ✅ | Accept header validation |
| Correlation IDs | ✅ | `Request-Id` + `Correlation-Id` (RFC 6648) |
| Distributed Tracing | ✅ | OpenTelemetry + W3C Trace Context |
| Health Checks | ✅ | `/liveness`, `/readiness`, `/health` |
| Circuit Breaker | ✅ | Sony gobreaker on gRPC client |
| Retry Mechanism | ✅ | Exponential backoff with jitter |
| Timeout Handling | ✅ | HTTP and gRPC timeouts |
| CORS | ✅ | Full support with preflight caching |
| Security Headers | ✅ | HSTS, CSP, X-Frame-Options, etc. |
| Structured Logging | ✅ | Zerolog with request context |
| Metrics/Telemetry | ✅ | OpenTelemetry (counters, histograms) |
| OpenAPI Docs | ✅ | Full spec with code generation |
| Rate Limiting | ⏳ | Headers defined, not implemented |
| Caching | ⏳ | ETag/Cache-Control headers defined |
| Idempotency | ⏳ | `Idempotency-Key` header defined |
| Compression | ⏳ | Accept-Encoding headers defined |

**Legend**: ✅ Implemented | ⏳ Planned

---

## API Gateway Features

### API Versioning

The API supports dual versioning strategies:

- **URL Path Versioning** (Primary): All endpoints are prefixed with `/v1/`
- **Header Versioning** (Alternative): Clients can specify `API-Version: v1` header

All responses include the `API-Version` header indicating the version used.

**Location**: `services/svc-api-gateway/internal/adapters/inbound/http/middleware/security_headers.go`

---

### Authentication & Authorization

#### PASETO v4 Authentication

Secure, stateless token-based authentication using PASETO v4 (Platform-Agnostic Security Tokens).

- Token format: `Bearer v4.public.{payload}.{signature}`
- Claims extraction with context injection
- Configurable skip paths for public endpoints (default: `/v1/health`, `/v1/liveness`, `/v1/readiness`)

#### Basic Authentication

Used for administrative endpoints like `/health` for detailed system information.

**Location**: `services/svc-api-gateway/internal/adapters/inbound/http/middleware/authentication.go`

---

### Request Validation

OpenAPI 3.0.3 specification-based validation:

- Path parameter validation
- Request body schema validation
- Header validation (Authorization, Content-Type, X-Request-Id, API-Version, etc.)
- Status-specific error handling (400, 401, 422)
- Error message sanitization to prevent information leakage

**Location**: `services/svc-api-gateway/internal/adapters/inbound/http/middleware/request_validator.go`

---

### Error Handling

Comprehensive error handling with structured responses:

```json
{
  "code": "DEVICE_NOT_FOUND",
  "message": "Device with ID '123' not found",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

Features:
- gRPC error mapping to domain errors (NotFound → 404, FailedPrecondition → business logic errors)
- Business rule validation errors
- Timeout handling (gRPC DeadlineExceeded)
- Panic recovery with stack trace logging
- OpenAPI-specified error schemas for: 400, 401, 404, 406, 409, 412, 422, 429, 500

**Locations**:
- `services/svc-api-gateway/internal/adapters/inbound/http/handlers/devices.go`
- `services/svc-api-gateway/internal/adapters/outbound/devices/error_mapper.go`
- `services/svc-api-gateway/internal/adapters/inbound/http/middleware/recovery.go`

---

### Pagination

Query parameter-based pagination with metadata:

| Parameter | Description | Default | Range |
|-----------|-------------|---------|-------|
| `page` | Page number (1-indexed) | 1 | 1+ |
| `size` | Items per page | 20 | 1-100 |

Response includes pagination metadata:

```json
{
  "data": [...],
  "pagination": {
    "page": 1,
    "size": 20,
    "totalItems": 150,
    "totalPages": 8,
    "hasNext": true,
    "hasPrevious": false
  }
}
```

**Location**: `services/svc-api-gateway/internal/adapters/inbound/http/handlers/devices.go`

---

### Filtering & Sorting

#### Filtering

| Parameter | Description | Example |
|-----------|-------------|---------|
| `brand` | Case-insensitive partial match | `?brand=apple` |
| `state` | Filter by device state | `?state=available` |

#### Sorting

| Parameter | Description | Example |
|-----------|-------------|---------|
| `sort` | Sort field with optional `-` prefix for descending | `?sort=-createdAt` |

Sortable fields: `name`, `brand`, `state`, `createdAt`, `updatedAt`

Default sort: `-createdAt` (newest first)

---

### Field Projection (Sparse Fieldsets)

Selective field inclusion to reduce payload size:

```
GET /v1/devices?fields=id,name,brand
GET /v1/devices?fields=id,name,links:(self)
```

Default fields: `id`, `name`, `brand`, `state`

Available fields: `id`, `name`, `brand`, `state`, `createdAt`, `updatedAt`, `links`

---

### HATEOAS (Hypermedia Links)

Responses include hypermedia links for resource navigation:

```json
{
  "id": "123",
  "name": "iPhone 15",
  "links": {
    "self": "/v1/devices/123"
  }
}
```

**Location**: `services/svc-api-gateway/internal/adapters/inbound/http/handlers/devices.go`

---

### Content Negotiation

- `Accept` header validation (defaults to `application/json`)
- Returns `406 Not Acceptable` for unsupported media types
- Currently supports JSON only

---

### Correlation IDs & Distributed Tracing

All custom headers follow RFC 6648 (no `X-` prefix).

#### Request ID

- Header: `Request-Id`
- Always generated server-side (per-request unique identifier)
- Propagated through entire request lifecycle
- Passed to downstream gRPC services via metadata

#### Correlation ID

- Header: `Correlation-Id`
- Can be provided by client or generated server-side
- Used to trace requests across multiple services
- Persists across service boundaries

#### W3C Trace Context

- `traceparent` header support
- `tracestate` header support
- OpenTelemetry integration for distributed tracing

**Locations**:
- `services/svc-api-gateway/internal/adapters/inbound/http/middleware/request_id.go`
- `services/svc-api-gateway/internal/adapters/inbound/http/middleware/tracer.go`
- `services/svc-api-gateway/internal/adapters/outbound/devices/interceptors.go`

---

### Health Checks

Three-tier health check system for Kubernetes deployments:

| Endpoint | Purpose | Auth Required |
|----------|---------|---------------|
| `/v1/liveness` | Is the service running? | No |
| `/v1/readiness` | Can the service handle requests? | No |
| `/v1/health` | Detailed system health | Basic Auth |

#### Health Response

```json
{
  "status": "up",
  "version": "1.0.0",
  "buildVersion": "abc123",
  "goVersion": "go1.25",
  "uptime": "2h30m15s",
  "startTime": "2024-01-15T08:00:00Z",
  "system": {
    "cpuCores": 8,
    "goroutines": 42
  },
  "dependencies": [
    {
      "name": "svc-devices",
      "status": "up",
      "latency": "5ms"
    }
  ]
}
```

**Location**: `services/svc-api-gateway/internal/adapters/inbound/http/handlers/devices.go`

---

### Circuit Breaker

Protection against cascading failures using Sony's gobreaker library:

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | true | Enable/disable circuit breaker |
| `maxRequests` | 5 | Max requests in half-open state |
| `interval` | 60s | Interval to clear failure counts |
| `timeout` | 30s | Time to wait before half-open |
| `failureThreshold` | 5 | Failures before opening circuit |

States: Closed → Open → Half-Open → Closed

**Location**: `services/svc-api-gateway/internal/adapters/outbound/devices/client.go`

---

### Retry Mechanism

Automatic retry with exponential backoff and jitter:

| Setting | Default | Description |
|---------|---------|-------------|
| `baseDelay` | 1s | Initial delay |
| `multiplier` | 1.6 | Delay multiplier per retry |
| `jitter` | 0.2 | Random jitter factor |
| `maxDelay` | 10s | Maximum delay cap |
| `maxRetries` | 3 | Maximum retry attempts |

Retryable gRPC status codes: `Unavailable`, `ResourceExhausted`, `Aborted`

**Location**: `services/svc-api-gateway/internal/adapters/outbound/devices/interceptors.go`

---

### Timeout Handling

Multi-layer timeout configuration:

| Timeout | Default | Description |
|---------|---------|-------------|
| HTTP Read | 15s | Max time to read request |
| HTTP Write | 15s | Max time to write response |
| HTTP Idle | 60s | Keep-alive timeout |
| gRPC Client | 30s | Downstream service timeout |
| Shutdown | 30s | Graceful shutdown timeout |

**Location**: `services/svc-api-gateway/internal/config/settings.go`

---

### CORS (Cross-Origin Resource Sharing)

Full CORS support with preflight caching:

| Setting | Value |
|---------|-------|
| Allowed Origins | Configurable (wildcard supported) |
| Allowed Methods | GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD |
| Allowed Headers | Authorization, Content-Type, Request-Id, Correlation-Id, API-Version, If-Match, If-None-Match, traceparent, tracestate, Idempotency-Key |
| Exposed Headers | Request-Id, Correlation-Id, RateLimit-*, ETag, Location |
| Preflight Cache | 86400s (24 hours) |

**Location**: `services/svc-api-gateway/internal/adapters/inbound/http/middleware/security_headers.go`

---

### Security Headers

All responses include security headers:

| Header | Value | Purpose |
|--------|-------|---------|
| `X-Content-Type-Options` | nosniff | Prevent MIME sniffing |
| `X-Frame-Options` | DENY | Prevent clickjacking |
| `X-XSS-Protection` | 1; mode=block | XSS protection |
| `Strict-Transport-Security` | max-age=31536000; includeSubDomains | HSTS |
| `Content-Security-Policy` | default-src 'self' | CSP |
| `Referrer-Policy` | strict-origin-when-cross-origin | Referrer control |
| `Permissions-Policy` | camera=(), microphone=(), geolocation=() | Feature policy |

**Location**: `services/svc-api-gateway/internal/adapters/inbound/http/middleware/security_headers.go`

---

### Structured Logging

Zerolog-based structured logging with:

- Request context (request_id, method, path, query, remote_addr, user_agent)
- Automatic log level based on HTTP status:
  - 2xx → Info
  - 4xx → Warn
  - 5xx → Error
- Panic recovery with stack traces
- Command/query decorator logging
- Configurable access log filtering (skip health checks)

**Locations**:
- `services/svc-api-gateway/internal/adapters/inbound/http/middleware/logging.go`
- `services/svc-api-gateway/shared/decorator/logging.go`

---

### Metrics & Telemetry

OpenTelemetry-based observability:

#### HTTP Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `http_requests_total` | Counter | method, path, status |
| `http_request_duration_seconds` | Histogram | method, path, status |
| `http_request_size_bytes` | Histogram | method, path |
| `http_response_size_bytes` | Histogram | method, path, status |

#### Operation Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `commands.{action}.duration` | Histogram | Command execution time |
| `commands.{action}.success` | Counter | Successful commands |
| `commands.{action}.failure` | Counter | Failed commands |
| `queries.{action}.duration` | Histogram | Query execution time |
| `queries.{action}.success` | Counter | Successful queries |
| `queries.{action}.failure` | Counter | Failed queries |

Configuration:
- Configurable via `METRICS_ENABLED` and `TRACES_ENABLED`
- Configurable trace sampling ratio
- gRPC exporter (default)

**Locations**:
- `services/svc-api-gateway/internal/adapters/inbound/http/middleware/metrics.go`
- `services/svc-api-gateway/shared/decorator/metrics.go`

---

### OpenAPI Documentation

Full OpenAPI 3.0.3 specification with:

- Detailed endpoint documentation
- Request/response examples
- Parameter and header documentation
- Security scheme definitions (PASETO, BasicAuth)
- Error response schemas for all HTTP status codes
- Server variables for environment-specific URLs
- Code generation via oapi-codegen v2

**Location**: `docs/openapi-spec/devices/v1/svc-api-gateway.yaml`

---

## Planned Features

The following features are documented in the OpenAPI specification but not yet implemented:

### Rate Limiting

Headers defined (RFC 6648 compliant):
- `RateLimit-Limit`: Maximum requests per window
- `RateLimit-Remaining`: Remaining requests in window
- `RateLimit-Reset`: Window reset timestamp
- `Retry-After`: Seconds until retry allowed (HTTP standard)

### Caching

Headers defined:
- `ETag`: Resource version tag
- `Cache-Control`: Caching directives
- `Last-Modified`: Resource modification time
- `If-None-Match`: Conditional GET
- `If-Match`: Conditional PUT/PATCH

### Idempotency

Header defined:
- `Idempotency-Key`: UUID v7 for POST request deduplication (24-hour TTL)

### Compression

Headers defined:
- `Accept-Encoding`: gzip, deflate, br
- `Content-Encoding`: Response compression
