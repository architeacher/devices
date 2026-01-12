# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
