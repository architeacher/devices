```bash
    ____            _
   / __ \___ _   __(_)_______  _____
  / / / / _ \ | / / / ___/ _ \/ ___/
 / /_/ /  __/ |/ / / /__/  __(__  )
/_____/\___/|___/_/\___/\___/____/

```

REST API for device management with hexagonal architecture and CQS.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Local URLs](#local-urls)
- [Project Structure](#project-structure)
- [Available Commands](#available-commands)
- [Testing](#testing)
  - [Unit Tests](#unit-tests)
  - [Integration Tests](#integration-tests)
- [API Examples](#api-examples)
  - [Create a Device](#create-a-device)
  - [Update a Device](#update-a-device)
  - [List Devices](#list-devices)
  - [Get a Device by ID](#get-a-device-by-id)
  - [Patch a Device (Partial Update)](#patch-a-device-partial-update)
  - [Delete a Device](#delete-a-device)
  - [List Devices with Filtering](#list-devices-with-filtering)
  - [HEAD Requests](#head-requests)
  - [OPTIONS Request (CORS Preflight)](#options-request-cors-preflight)
- [API Gateway Capabilities](#api-gateway-capabilities)
  - [Implemented Features](#implemented-features)
  - [Legend](#legend)
  - [Future Improvements](#future-improvements)
- [Documentation](#documentation)

## Prerequisites

- Go 1.25+
- Docker & Docker Compose
- mkcert (for local TLS certificates)

## Quick Start

```bash
# Initialize project (env files, hosts, certificates, API generation)
make init

# Start development environment
make start
```

## Local URLs

| Service | URL |
|---------|-----|
| API | https://api.devices.dev |
| API Documentation | https://docs.devices.dev |
| Traefik Dashboard | https://traefik.devices.dev |
| Vault UI | https://vault.devices.dev |

## Project Structure

```
devices/
├── services/
│   ├── api-gateway/    # REST API (public-facing)
│   └── svc-devices/    # Internal service (gRPC + PostgreSQL)
├── docs/
│   └── openapi-spec/   # OpenAPI 3.1 specifications
├── build/
│   ├── mk/             # Modular Makefiles
│   └── oapi/           # Code generation config
└── deployment/
    └── docker/         # Docker/Traefik configuration
```

## Available Commands

```bash
make help       # Show all available targets
make start      # Start Docker containers
make restart    # Restart Docker containers
make destroy    # Stop and remove containers
make stats      # Show code statistics
make todo       # Find TODO items in codebase
```

## Testing

### Unit Tests

```bash
# Run all unit tests with race detection
make test-unit

# Run specific service tests
cd services/svc-devices && go test -v -race ./...
cd services/svc-api-gateway && go test -v -race ./...
```

### Integration Tests

Integration tests use `testcontainers-go` to spin up real PostgreSQL containers.

```bash
# Run integration tests (requires Docker)
make test-integration

# Run integration tests for specific service
cd services/svc-devices && go test -v -race -tags=integration ./...
```

## API Examples

### Create a Device

```bash
curl -s -X POST https://api.devices.dev/v1/devices \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer v4.public.test-token" \
  -H "Idempotency-Key: 550e8400-e29b-41d4-a716-446655440001" \
  -d '{
    "name": "iPhone 15 Pro",
    "brand": "Apple",
    "state": "available"
  }' | jq
```

**Response (201 Created):**

```json
{
  "data": {
    "id": "019b3915-3302-7a6b-843b-9cca8343bf08",
    "name": "iPhone 15 Pro",
    "brand": "Apple",
    "state": "available",
    "createdAt": "2025-12-20T00:07:29.282462909Z",
    "updatedAt": "2025-12-20T00:07:29.282462909Z",
    "links": {
      "self": "/v1/devices/019b3915-3302-7a6b-843b-9cca8343bf08"
    }
  }
}
```

### Update a Device

```bash
curl -s -X PUT https://api.devices.dev/v1/devices/019b3915-3302-7a6b-843b-9cca8343bf08 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer v4.public.test-token" \
  -d '{
    "name": "iPhone 15 Pro Max",
    "brand": "Apple",
    "state": "in-use"
  }' | jq
```

**Response (200 OK):**

```json
{
  "data": {
    "id": "019b3915-3302-7a6b-843b-9cca8343bf08",
    "name": "iPhone 15 Pro Max",
    "brand": "Apple",
    "state": "in-use",
    "createdAt": "2025-12-20T00:07:29.282462Z",
    "updatedAt": "2025-12-20T00:07:37.775526891Z",
    "links": {
      "self": "/v1/devices/019b3915-3302-7a6b-843b-9cca8343bf08"
    }
  }
}
```

### List Devices

```bash
curl -s "https://api.devices.dev/v1/devices?brand=Apple&state=available&sort=-createdAt&page=1&size=20" \
  -H "Authorization: Bearer v4.public.test-token" | jq
```

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "019b3915-3302-7a6b-843b-9cca8343bf08",
      "name": "iPhone 15 Pro Max",
      "brand": "Apple",
      "state": "in-use",
      "createdAt": "2025-12-20T00:07:29.282462Z",
      "updatedAt": "2025-12-20T00:07:37.775526Z",
      "links": {
        "self": "/v1/devices/019b3915-3302-7a6b-843b-9cca8343bf08"
      }
    },
    {
      "id": "019b38a1-4e59-7c6a-aaf0-0c60fccccf89",
      "name": "Samsung Galaxy S24",
      "brand": "Samsung",
      "state": "available",
      "createdAt": "2025-12-19T22:00:54.105486Z",
      "updatedAt": "2025-12-19T22:00:54.105486Z",
      "links": {
        "self": "/v1/devices/019b38a1-4e59-7c6a-aaf0-0c60fccccf89"
      }
    }
  ],
  "pagination": {
    "page": 1,
    "size": 20,
    "totalItems": 2,
    "totalPages": 1,
    "hasNext": false,
    "hasPrevious": false
  }
}
```

### Get a Device by ID

```bash
curl -s https://api.devices.dev/v1/devices/019b3915-3302-7a6b-843b-9cca8343bf08 \
  -H "Authorization: Bearer v4.public.test-token" | jq
```

**Response (200 OK):**

```json
{
  "data": {
    "id": "019b3915-3302-7a6b-843b-9cca8343bf08",
    "name": "iPhone 15 Pro Max",
    "brand": "Apple",
    "state": "in-use",
    "createdAt": "2025-12-20T00:07:29.282462Z",
    "updatedAt": "2025-12-20T00:07:37.775526Z",
    "links": {
      "self": "/v1/devices/019b3915-3302-7a6b-843b-9cca8343bf08"
    }
  }
}
```

### Patch a Device (Partial Update)

```bash
curl -s -X PATCH https://api.devices.dev/v1/devices/019b3915-3302-7a6b-843b-9cca8343bf08 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer v4.public.test-token" \
  -d '{
    "state": "inactive"
  }' | jq
```

**Response (200 OK):**

```json
{
  "data": {
    "id": "019b3915-3302-7a6b-843b-9cca8343bf08",
    "name": "iPhone 15 Pro Max",
    "brand": "Apple",
    "state": "inactive",
    "createdAt": "2025-12-20T00:07:29.282462Z",
    "updatedAt": "2025-12-20T00:28:03.759538Z",
    "links": {
      "self": "/v1/devices/019b3915-3302-7a6b-843b-9cca8343bf08"
    }
  }
}
```

### Delete a Device

```bash
curl -s -X DELETE https://api.devices.dev/v1/devices/019b3915-3302-7a6b-843b-9cca8343bf08 \
  -H "Authorization: Bearer v4.public.test-token" | jq
```

**Response (204 No Content):** Empty response body

**Note:** Devices with state `in-use` cannot be deleted (returns 409 Conflict).

### List Devices with Filtering

Filter by brand and state, with sorting:

```bash
curl -s "https://api.devices.dev/v1/devices?brand=Apple&state=available&sort=-createdAt" \
  -H "Authorization: Bearer v4.public.test-token" | jq
```

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "019b3915-3302-7a6b-843b-9cca8343bf08",
      "name": "iPhone 15 Pro",
      "brand": "Apple",
      "state": "available",
      "createdAt": "2025-12-20T00:07:29.282462Z",
      "updatedAt": "2025-12-20T00:07:29.282462Z",
      "links": {
        "self": "/v1/devices/019b3915-3302-7a6b-843b-9cca8343bf08"
      }
    }
  ],
  "pagination": {
    "page": 1,
    "size": 20,
    "totalItems": 1,
    "totalPages": 1,
    "hasNext": false,
    "hasPrevious": false
  }
}
```

**Available Query Parameters:**

| Parameter | Description | Example |
|-----------|-------------|---------|
| `brand` | Filter by brand (case-insensitive) | `?brand=Apple` |
| `state` | Filter by state | `?state=available` |
| `sort` | Sort field (prefix `-` for descending) | `?sort=-createdAt` |
| `page` | Page number (1-indexed) | `?page=2` |
| `size` | Items per page (1-100) | `?size=10` |

### HEAD Requests

Check if a device exists (metadata only, no body):

```bash
curl --head https://api.devices.dev/v1/devices/019b3915-3302-7a6b-843b-9cca8343bf08 \
  -H "Authorization: Bearer v4.public.test-token"
```

**Response Headers:**

```
HTTP/2 200
api-version: v1
x-total-count: 1
```

### OPTIONS Request (CORS Preflight)

```bash
curl -w "%{http_code}\n" -X OPTIONS https://api.devices.dev/v1/devices
```

**Response (204 No Content):**

```
HTTP/2 204
allow: GET, POST, HEAD, OPTIONS
api-version: v1
```

## API Gateway Capabilities

### Implemented Features

#### Request Processing & Routing

| Feature | Status | Description |
|---------|--------|-------------|
| **Parameter Validation** | ✅ Full | Deep request validation using OpenAPI 3.0.3 schema via `kin-openapi/openapi3filter` (query params, path params, request body) with sanitized error messages |
| **Dynamic Request Routing** | ⚠️ Partial | Path-based routing with Chi router v5; service discovery not yet integrated |
| **Protocol Conversion** | ✅ Full | HTTP/1.1 → gRPC translation with bidirectional message mapping and error code translation |

#### Security & Access Control

| Feature | Status | Description |
|---------|--------|-------------|
| **Authentication** | ⚠️ Partial | PASETO v4 token format validation; cryptographic verification pending |
| **Data Encryption** | ⚠️ Partial | TLS 1.2+ with mTLS for gRPC backend; HTTP TLS via Traefik reverse proxy |
| **Security Headers** | ✅ Full | CORS, CSP, HSTS, X-Frame-Options, X-Content-Type-Options, Referrer-Policy, Permissions-Policy |

#### Traffic Management & Resilience

| Feature | Status | Description |
|---------|--------|-------------|
| **Rate Limiting** | ✅ Full | GCRA-based rate limiting via `throttled/throttled/v2` with KeyDB storage (per IP/user/global), RFC-compliant headers (`RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`), graceful degradation |
| **Circuit Breaker** | ✅ Full | Per-service failure detection using `sony/gobreaker/v2` with half-open state recovery |

#### Performance Optimization

| Feature | Status | Description |
|---------|--------|-------------|
| **Response Caching** | ⚠️ Partial | ETag/If-None-Match headers supported; KeyDB-backed caching not implemented |

#### Observability & Operations

| Feature | Status | Description |
|---------|--------|-------------|
| **Error Handling** | ✅ Full | Retry logic with exponential backoff (`cenkalti/backoff/v4`), configurable jitter, timeout management |
| **Logging** | ✅ Full | Structured access logs with Zerolog (request ID, method, path, status, duration, conditional log levels) |
| **Metrics** | ✅ Full | OpenTelemetry metrics (latency, throughput, errors, request/response sizes) with decorator pattern |
| **Distributed Tracing** | ✅ Full | OpenTelemetry tracing with Jaeger integration, span propagation, and command/query instrumentation |

#### Developer Experience

| Feature | Status | Description |
|---------|--------|-------------|
| **Hot Reload** | ✅ Full | Air-based live reloading for both services - automatically rebuilds and restarts on source file changes |

### Legend

- ✅ **Full** - Feature is production-ready
- ⚠️ **Partial** - Core functionality exists, enhancements planned

### Future Improvements

The following features are planned for future releases:

#### Security

| Feature | Priority | Description |
|---------|----------|-------------|
| PASETO v4 Verification | High | Complete cryptographic token verification with key rotation |
| RBAC Authorization | High | Role-based access control with policy enforcement (OPA/Casbin) |
| Request Signing | Medium | HTTP message signatures (RFC 9421) for request integrity |
| Allow/Deny Lists | Medium | IP/CIDR/user-agent filtering with whitelist/blacklist support |
| Secrets Rotation | Medium | Automatic Vault secret rotation with zero-downtime |
| Vulnerability Scanning | Low | Container image scanning (Trivy) and dependency auditing |

#### Traffic Management

| Feature | Priority | Description                                                            |
|---------|----------|------------------------------------------------------------------------|
| Service Discovery | High | Dynamic backend registration with Consul/etcd integration              |
| Load Balancing | Medium |  Multiple strategies (round-robin, weighted, least-connections, consistent hashing)           |
| SSE Streaming | Low | Convert gRPC server streams to Server-Sent Events for real-time updates |
| API Composition | Low | Combine multiple gRPC calls into single HTTP response (scatter-gather pattern)                   |
| Load Shedding | Low | Adaptive probabilistic request dropping under overload                 |

#### DevOps & Infrastructure

| Feature | Priority | Description |
|---------|----------|-------------|
| CI/CD Pipelines | High | GitHub Actions for build, test, lint, security scan, and deploy |
| Linting | High | golangci-lint with custom rules, pre-commit hooks |
| Kubernetes Deployment | High | Helm charts, HPA, PDB, NetworkPolicies, Kustomize overlays |
| Container Registry | Medium | Multi-arch images (ARM64/AMD64) with GitHub Container Registry |
| GitOps | Medium | ArgoCD/Flux for declarative deployments |
| Infrastructure as Code | Medium | Terraform modules for cloud resources |

#### Testing & Quality

| Feature | Priority | Description |
|---------|----------|-------------|
| E2E Tests | High | Playwright/k6 for API end-to-end testing |
| Contract Testing | Medium | Pact for consumer-driven contract testing |
| Load Testing | Medium | k6/Gatling scripts with performance baselines |
| Chaos Engineering | Low | Chaos Mesh/Litmus for resilience testing |
| Mutation Testing | Low | go-mutesting for test quality validation |

#### Observability

| Feature | Priority | Description |
|---------|----------|-------------|
| Grafana Dashboards | Medium | Pre-built dashboards for API metrics and traces |
| Alerting Rules | Medium | Prometheus alerting with PagerDuty/Slack integration |
| SLO/SLI Definitions | Medium | Service level objectives with error budgets |
| Log Aggregation | Low | Loki/Elasticsearch for centralized logging |

#### Developer Experience

| Feature | Priority | Description |
|---------|----------|-------------|
| SDK Generation | Medium | Auto-generated Go/TypeScript clients from OpenAPI spec |
| Dev Containers | Low | VS Code devcontainer for consistent environments |
| API Mocking | Low | Prism/WireMock for frontend development |

## Documentation

- [Architecture](docs/architecture.md) - ADRs and system diagrams
- [API Specification](docs/openapi-spec/devices/v1/svc-api-gateway.yaml) - OpenAPI spec
