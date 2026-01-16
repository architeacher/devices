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
| Pagination | ✅ | Offset + cursor-based (keyset) pagination |
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
| Rate Limiting | ✅ | GCRA algorithm with RFC-compliant headers |
| Idempotency | ✅ | `Idempotency-Key` with KeyDB-backed storage |
| Deprecation Headers | ✅ | RFC 8594 Sunset headers for API versioning |
| Caching | ✅ | Cache-Aside pattern with ETag/conditional GET |
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

The API supports two pagination strategies:

#### Offset-Based Pagination (Default)

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

#### Cursor-Based Pagination (Keyset)

For large datasets and real-time data, cursor-based pagination provides consistent results:

| Parameter | Description | Example |
|-----------|-------------|---------|
| `cursor` | Opaque cursor token | `eyJmIjoiY3JlYXRlZF9hdCIsInYiOi...` |
| `size` | Items per page | 20 |

Response includes cursor metadata:

```json
{
  "data": [...],
  "meta": {
    "cursors": {
      "next": "eyJmIjoiY3JlYXRlZF9hdCIsInYiOi...",
      "previous": "eyJmIjoiY3JlYXRlZF9hdCIsInYiOi..."
    },
    "hasNext": true,
    "hasPrevious": false
  }
}
```

**Cursor Structure** (base64-encoded JSON):
- `f`: Sort field (e.g., `created_at`, `name`)
- `v`: Field value at cursor position
- `id`: Device ID for tie-breaking
- `d`: Direction (`next` or `prev`)

**Advantages over offset pagination:**
- No skipped/duplicate items when data changes
- Consistent performance regardless of page depth
- Works well with real-time data streams

**Locations**:
- `services/svc-api-gateway/internal/adapters/inbound/http/handlers/devices.go`
- `services/svc-devices/internal/domain/model/cursor.go`

---

### Filtering & Sorting

#### Filtering

| Parameter | Description | Example |
|-----------|-------------|---------|
| `brand` | Filter by brand(s), comma-separated for OR logic | `?brand=Apple,Samsung` |
| `state` | Filter by state(s), comma-separated for OR logic | `?state=available,inactive` |

**Multi-value filtering:**
- Comma-separated values within a field use **OR** logic: `?brand=Apple,Samsung` matches devices with brand "Apple" OR "Samsung"
- Multiple filter parameters use **AND** logic: `?brand=Apple&state=available` matches Apple devices that are available
- Maximum 10 brands and 3 states per request

**Generated SQL:**
```sql
SELECT * FROM devices
WHERE brand IN ('Apple', 'Samsung') AND state IN ('available')
ORDER BY created_at DESC
LIMIT 20 OFFSET 0
```

#### Sorting

| Parameter | Description | Example |
|-----------|-------------|---------|
| `sort` | Sort field with optional `-` prefix for descending | `?sort=-createdAt` |

Sortable fields: `name`, `brand`, `state`, `createdAt`, `updatedAt`

Default sort: `-createdAt` (newest first)

---

### Future: Advanced Search Endpoint

For complex query scenarios beyond comma-separated filters, a dedicated search endpoint is planned:

#### Option A: RSQL via GET

```
GET /v1/devices/search?q=brand=in=(Apple,Samsung);state==available
```

RSQL provides a query language for REST APIs with operators like `==`, `!=`, `=in=`, `=out=`, `=gt=`, `=lt=`, `;` (AND), `,` (OR).

#### Option B: Elasticsearch/Lucene-style via POST

```http
POST /v1/devices/search
Content-Type: application/json

{
  "query": {
    "bool": {
      "must": [{"terms": {"brand": ["Apple", "Samsung"]}}],
      "should": [{"term": {"state": "available"}}]
    }
  }
}
```

This approach supports complex nested queries, full-text search, and aggregations.

**Status:** Planned for future release when use cases require advanced search capabilities.

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

**Location**: `docs/contracts/openapi/devices/v1/specs.yaml`

---

### Rate Limiting

Distributed rate limiting using GCRA (Generic Cell Rate Algorithm) with Redis/KeyDB storage:

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | true | Enable/disable rate limiting |
| `requestsPerSecond` | 10 | Request rate quota |
| `burstSize` | 20 | Burst capacity |
| `enableIPLimiting` | true | Rate limit by IP address |
| `enableUserLimiting` | true | Rate limit by user (PASETO claims) |
| `maxKeys` | 1000 | Maximum keys in store |
| `skipPaths` | /health | Paths to exclude |
| `gracefulDegraded` | true | Allow requests on store failure |

RFC-compliant response headers:
- `RateLimit-Limit`: Maximum requests per window
- `RateLimit-Remaining`: Remaining requests in window
- `RateLimit-Reset`: Window reset timestamp (Unix epoch)
- `Retry-After`: Seconds until retry allowed (on 429 responses)

Key generation strategy (combined IP + user):
- Authenticated requests: `ip:{addr}|user:{subject}`
- Unauthenticated requests: `ip:{addr}`
- Fallback: `global` if neither enabled

**Location**: `services/svc-api-gateway/internal/adapters/inbound/http/middleware/throttled_rate_limiting.go`

---

### Idempotency

Request deduplication for POST operations using `Idempotency-Key` header with KeyDB-backed storage:

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | true | Enable/disable idempotency checks |
| `headerName` | Idempotency-Key | Header name for idempotency key |
| `replayedHeader` | Idempotency-Replayed | Header indicating cached response |
| `requiredMethods` | POST | HTTP methods requiring idempotency |
| `cacheTTL` | 24h | How long to store responses |
| `lockTTL` | 30s | Lock timeout for in-progress requests |
| `gracefulDegraded` | true | Allow requests on cache failure |

#### Key Validation

- Must be a valid UUID (v4 or v7)
- Invalid keys return `400 Bad Request` with `INVALID_IDEMPOTENCY_KEY` code

#### Behavior

1. **No key provided**: Request processed normally (no idempotency)
2. **New key**: Request processed, response cached for `cacheTTL`
3. **Existing key (completed)**: Cached response returned with `Idempotency-Replayed: true`
4. **Existing key (in-progress)**: Returns `409 Conflict` with `REQUEST_IN_PROGRESS` code

#### Response Caching

Only successful responses (2xx status codes) are cached. Cached data includes:
- Status code
- Response headers
- Response body
- Creation timestamp

**Location**: `services/svc-api-gateway/internal/adapters/inbound/http/middleware/idempotency.go`

---

### Deprecation Headers (RFC 8594)

API versioning support with RFC 8594 compliant deprecation headers:

| Header | Description | Example |
|--------|-------------|---------|
| `Deprecation` | Indicates API is deprecated | `true` |
| `Sunset` | Date when API will be removed | `Sat, 01 Jun 2025 00:00:00 GMT` |
| `Link` | Points to successor version | `</v2/devices>; rel="successor-version"` |

#### Configuration

| Setting | Description | Example |
|---------|-------------|---------|
| `enabled` | Enable deprecation headers | `true` |
| `sunsetDate` | RFC3339 removal date | `2025-06-01T00:00:00Z` |
| `successorPath` | URL to new API version | `/v2/devices` |

#### Usage

When an API version is being deprecated:
1. Set `enabled: true` to start sending deprecation headers
2. Set `sunsetDate` to inform clients when the API will be removed
3. Set `successorPath` to guide clients to the new version

Clients can monitor these headers to plan their migration before the sunset date.

**Location**: `services/svc-api-gateway/internal/adapters/inbound/http/middleware/sunset.go`

---

### Caching

Device data caching with the Cache-Aside pattern at the API Gateway layer, using KeyDB for storage.

#### Architecture

The caching layer is implemented at the **Query Decorator** level (not HTTP middleware) for several reasons:
- Cache keys are semantic (device ID) rather than URL-based
- Easy invalidation by ID (`InvalidateDevice(id)`)
- Domain objects are serialized once, not per-request
- Full observability (all requests logged, metriced, traced including cache hits)

**Decorator Order**: `logging → metrics → tracing → cache → base`

#### Cache-Aside Pattern

```
Request → Check Cache → Hit? → Return cached data
                     → Miss? → Query backend → Cache result → Return data
```

#### Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | true | Enable/disable device caching |
| `deviceTTL` | 5m | TTL for individual device cache |
| `listTTL` | 1m | TTL for device list cache |
| `maxAge` | 60 | Cache-Control max-age seconds |
| `staleWhileRevalidate` | 30 | Stale-while-revalidate seconds |

Environment variables:
- `DEVICES_CACHE_ENABLED`
- `DEVICES_CACHE_DEVICE_TTL`
- `DEVICES_CACHE_LIST_TTL`
- `DEVICES_CACHE_MAX_AGE`
- `DEVICES_CACHE_STALE_REVALIDATE`

#### Response Headers

| Header | Value Example | Purpose |
|--------|---------------|---------|
| `Cache-Status` | `HIT`, `MISS`, `BYPASS` | Indicates cache result |
| `Cache-Key` | `device:v1:550e8400...` | Debug: cache key used |
| `Cache-TTL` | `287` | Remaining TTL in seconds |
| `ETag` | `"a1b2c3d4e5f6"` | Resource version for conditional GET |
| `Cache-Control` | `private, max-age=60, stale-while-revalidate=30` | Client caching directive |
| `Last-Modified` | `Tue, 13 Jan 2026 01:00:00 GMT` | Resource modification time |

#### ETag Generation

ETags are generated using xxhash for high performance:
- Strong ETags: `"abc123def456"` (quoted hex)
- Weak ETags: `W/"abc123def456"` (optional for list responses)

#### Conditional GET

The conditional GET middleware handles `If-None-Match` headers:
1. Client sends `If-None-Match: "abc123"`
2. Server generates ETag for current response
3. If match: Returns `304 Not Modified` (no body)
4. If no match: Returns full response with new ETag

#### Cache Invalidation

| Operation | Device Cache | List Cache |
|-----------|-------------|------------|
| Create | - | Invalidate all |
| Update | Invalidate ID | Invalidate all |
| Patch | Invalidate ID | Invalidate all |
| Delete | Invalidate ID | Invalidate all |

Invalidation runs asynchronously (goroutine) to avoid blocking responses.

#### Cache Key Patterns

- Individual device: `device:v1:{uuid}`
- Device list: `devices:list:v1:{filter_hash}`

Filter hashes use SHA-256 with sorted arrays for consistent keys regardless of parameter order.

#### Admin Operations

Internal cache management endpoints (not exposed on public API):
- `DELETE /admin/cache/devices` - Purge all device caches
- `DELETE /admin/cache/devices/{id}` - Purge specific device
- `DELETE /admin/cache/devices/lists` - Purge all list caches
- `GET /admin/cache/health` - Check cache health

Makefile targets:
```bash
make cache-purge          # Purge all device caches
make cache-purge-lists    # Purge list caches only
make cache-purge-device ID=<uuid>  # Purge specific device
make cache-stats          # Show cache statistics
make cache-keys           # List all device cache keys
```

**Locations**:
- `services/svc-api-gateway/internal/adapters/repos/devices_cache_repository.go`
- `services/svc-api-gateway/internal/adapters/inbound/http/middleware/etag.go`
- `services/svc-api-gateway/internal/adapters/inbound/http/middleware/conditional.go`
- `pkg/decorator/caching.go`

---

## Planned Features

The following features are documented in the OpenAPI specification but not yet implemented:

### Compression

Headers defined:
- `Accept-Encoding`: gzip, deflate, br
- `Content-Encoding`: Response compression
