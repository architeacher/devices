# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- HTTP response caching integration with ETag generation and 304 Not Modified responses
- ConditionalGET middleware registration in HTTP pipeline after compression
- Cache-Control headers with configurable max-age and stale-while-revalidate directives
- Separate TTL configuration for single device (60s/30s) and list endpoints (30s/15s)
- Vary header with Accept, Authorization, and Accept-Encoding for cache variance
- Last-Modified header on device responses using updatedAt timestamp
- Cache observability headers: Cache-Status (MISS/BYPASS), Cache-Key
- IsCacheBypassRequested helper for detecting Cache-Control: no-cache requests
- HTTPCachingEnabled configuration field for independent HTTP-layer caching control

## [Unreleased]

### Added

- HTTP response compression middleware with gzip, deflate, and brotli support
- Accept-Encoding header parsing with quality value (q=) support per RFC 7231
- Server preference order: gzip > brotli > deflate for equal quality values
- Configurable compression: level (1-9), minimum size threshold, content-type filtering
- Skip paths for health endpoints to avoid compression overhead
- Compression metrics: `http_compression_total`, `http_compression_original_bytes`, `http_compression_compressed_bytes`, `http_compression_ratio`
- Skip metrics: `http_compression_skipped_total` with reason attribute
- Structured logging for compression events (DEBUG for success, WARN for skips/errors)
- Vary: Accept-Encoding header for proper cache behavior
- 406 Not Acceptable response when a client rejects identity encoding with no alternatives
- Pooled compression writers (sync.Pool) for reduced GC pressure

### Changed

- Made gRPC message size limits configurable via `DEVICES_MAX_MESSAGE_SIZE` environment variable (default: 4 MiB)

### CI

- Extracted circuit breaker to shared `pkg/circuitbreaker` package with generic type support
- Removed `ErrCircuitOpen` from domain model - now uses `circuitbreaker.ErrCircuitOpen` and `circuitbreaker.ErrTooManyRequests`

## [0.5.0] - 2026-01-13

### Added

- Device data caching with the Cache-Aside pattern at API Gateway layer
- ETag generation using xxhash for conditional GET support
- Conditional GET middleware with `If-None-Match` header and 304 responses
- Query caching decorator with configurable TTL per query type
- Cache invalidation on Create, Update, Patch, and Delete operations
- Internal admin handlers for cache purge operations
- Makefile targets for cache management (`cache-purge`, `cache-purge-lists`, `cache-stats`)
- Cache status headers: `Cache-Status`, `Cache-Key`, `Cache-TTL`
- `Cache-Control` and `Last-Modified` response headers

### Changed

- Renamed `devices_repository.go` to `devices_postgres_repository.go` for clarity
- Added `DevicesCache` configuration section with TTL settings
- Command handlers now support cache invalidation via dependency injection

### Infrastructure

- Added `redis-cli` to API Gateway Docker image for cache management

## [0.4.0] - 2026-01-12

### Added

- Filtering and sorting support for device listing with multi-value OR logic
- GCRA-based rate limiting middleware with KeyDB storage and RFC-compliant headers
- Idempotency middleware with KeyDB-backed storage for POST request deduplication
- Correlation ID (`Correlation-Id`) and Request ID (`Request-Id`) header support (RFC 6648)
- Sunset middleware for API deprecation headers (RFC 8594: `Deprecation`, `Sunset`, `Link`)
- Cursor-based pagination model for keyset pagination support
- Response helpers for standardized HTTP responses with metadata
- Meta information schema for response envelopes

### Changed

- Updated pagination response to include cursor-based navigation support
- Enhanced criteria translator for advanced filtering operations

### Tests

- Comprehensive filtering and sorting test coverage for `svc-devices`

## [0.3.1] - 2025-12-20

### Added

- Repository layer tests for `svc-devices` with comprehensive coverage
- Unit tests using `pgxmock/v4` for SQL query verification
- Integration tests using `testcontainers-go` with PostgreSQL 18
- Interface-based dependency injection for repository testability (`PoolIface`)

## [0.3.0] - 2025-12-20

### Added

- `svc-devices` internal gRPC service with PostgreSQL persistence
- gRPC handlers for device CRUD operations and health checks
- Domain models: Device entity with State value object
- PostgreSQL repository implementation for device storage
- CQS use cases: CreateDevice, UpdateDevice, PatchDevice, DeleteDevice commands
- CQS use cases: GetDevice, ListDevices, health check queries
- Decorator pattern for cross-cutting concerns (logging, metrics, tracing)
- Configuration loading with Vault integration
- OpenTelemetry telemetry infrastructure
- Docker configuration with Air hot-reload for development
- Unit tests for handlers, domain models, commands, and queries

## [0.2.0] - 2025-12-18

### Added

- OpenAPI code generation with oapi-codegen v2 integration
- Automated HTTP handler generation via `make generate-api`
- HashiCorp Vault integration for secrets management
- KeyDB (Redis-compatible) caching infrastructure
- Vault initialization script with AppRole authentication
- Per-service environment configuration generation (`make init-services`)

## [0.1.0] - 2025-12-17

### Added

- OpenAPI 3.1 specification for Devices API
