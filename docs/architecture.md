# Architecture

## Architecture Decision Records

### ADR-001: Two-Service Architecture

**Context**: Separation of concerns between public API and internal business logic.

**Decision**: Split into two services:
- **api-gateway**: Public REST API, authentication, request validation
- **svc-devices**: Internal service with business logic and database access

**Consequences**:
- Clear responsibility separation
- Independent scaling
- gRPC for efficient inter-service communication

---

### ADR-002: API-First Development with OpenAPI

**Context**: Need consistent API contracts and documentation.

**Decision**: Use OpenAPI 3.1 specification as the source of truth:
- Define API contracts in YAML files
- Generate server stubs using oapi-codegen v2
- Serve Swagger UI for interactive documentation

**Consequences**:
- Single source of truth for API contracts
- Type-safe generated code
- Always up-to-date documentation

---

### ADR-003: PASETO for Authentication

**Context**: Need secure, stateless authentication for API access.

**Decision**: Use PASETO v4 (Platform-Agnostic Security Tokens):
- More secure defaults than JWT
- No algorithm confusion attacks
- Built-in versioning

**Consequences**:
- Stateless authentication
- Strong cryptographic defaults
- Token-based access control

---

### ADR-004: Traefik as Reverse Proxy

**Context**: Need TLS termination, routing, and observability for local development.

**Decision**: Use Traefik v3 as the entry point:
- TLS termination with local certificates
- Dynamic service discovery via Docker labels
- Built-in dashboard for debugging

**Consequences**:
- Consistent HTTPS in development and production
- Easy routing configuration
- Circuit breaker and rate limiting capabilities

---

### ADR-005: HashiCorp Vault for Secrets Management

**Context**: Need secure, centralized secrets management for all services.

**Decision**: Use HashiCorp Vault with AppRole authentication:
- Centralized secret storage with KV v2 engine
- Per-service policies with least-privilege access
- AppRole authentication for service-to-Vault communication
- Automated initialization via init container

**Consequences**:
- Secrets not stored in environment files or code
- Auditable access to sensitive data
- Dynamic credential rotation capability
- Service isolation through policies

---

### ADR-006: KeyDB for Caching

**Context**: Need high-performance caching layer for API responses and session data.

**Decision**: Use KeyDB (Redis-compatible) as the caching solution:
- Drop-in Redis replacement with better performance
- Persistent storage with periodic snapshots
- Password-protected access

**Consequences**:
- Reduced database load
- Faster response times for cached data
- Compatible with existing Redis clients

---

### ADR-007: Hexagonal Architecture (Ports and Adapters)

**Context**: Need a maintainable, testable architecture that isolates business logic from infrastructure concerns.

**Decision**: Adopt Hexagonal Architecture (also known as Ports and Adapters):
- **Domain Layer** (`domain/model/`): Core business entities and rules, no external dependencies
- **Ports** (`ports/`): Interface definitions that the domain exposes (inbound) or requires (outbound)
- **Adapters** (`adapters/`): Implementations that connect external systems to ports
  - Inbound adapters: HTTP handlers, gRPC handlers
  - Outbound adapters: Database repositories, external service clients
- **Use Cases** (`usecases/`): Application-specific business rules orchestrating domain and ports

**Consequences**:
- Business logic is isolated from frameworks and infrastructure
- Easy to test domain logic in isolation with mock adapters
- Flexible to swap implementations (e.g., change a database without touching business logic)
- Clear dependency direction: adapters depend on ports, never the reverse

---

### ADR-008: Command-Query Separation (CQS)

**Context**: Need clear separation between operations that modify state and those that only read state.

**Decision**: Apply [CQS pattern](https://en.wikipedia.org/wiki/Command%E2%80%93query_separation) in the use cases layer:
- **Commands** (`usecases/commands/`): Operations that change state (Create, Update, Delete)
- **Queries** (`usecases/queries/`): Operations that return data without side effects (Get, List)
- Each handler implements a single responsibility with a consistent interface
- Decorators (`shared/decorator/`) wrap handlers for cross-cutting concerns (logging, metrics, tracing)

**Consequences**:
- Clear intent: readers know immediately if an operation modifies the state
- Easier optimization: queries can be cached, commands trigger cache invalidation
- Simplified testing: commands and queries can be tested independently
- Natural fit for event sourcing or CQRS evolution if needed

---

## System Diagrams

### C4 Context Diagram

```mermaid
graph TB
    subgraph External
        User[API Consumer]
        OTEL[OpenTelemetry Collector]
    end

    subgraph System[Devices API System]
        API[Devices API]
    end

    User -->|HTTPS/REST| API
    API -->|Traces/Metrics| OTEL
```

### C4 Container Diagram

```mermaid
graph TB
    User[API Consumer]

    subgraph System[Devices System]
        Traefik[Traefik<br/>Reverse Proxy]
        Gateway[api-gateway<br/>Go/Chi]
        Devices[svc-devices<br/>Go/gRPC]
        DB[(PostgreSQL)]
        Vault[(Vault<br/>Secrets)]
        KeyDB[(KeyDB<br/>Cache)]
    end

    User -->|HTTPS| Traefik
    Traefik -->|HTTP| Gateway
    Gateway -->|gRPC| Devices
    Gateway -->|Secrets| Vault
    Gateway -->|Cache| KeyDB
    Devices -->|SQL| DB
    Devices -->|Secrets| Vault
```

### Service Communication Sequence

```mermaid
sequenceDiagram
    participant Client
    participant Traefik
    participant Gateway as api-gateway
    participant Devices as svc-devices
    participant DB as PostgreSQL

    Client->>Traefik: HTTPS Request
    Traefik->>Gateway: HTTP (TLS terminated)
    Gateway->>Gateway: Validate PASETO Token
    Gateway->>Devices: gRPC Call
    Devices->>DB: SQL Query
    DB-->>Devices: Result
    Devices-->>Gateway: gRPC Response
    Gateway-->>Traefik: HTTP Response
    Traefik-->>Client: HTTPS Response
```

### Data Flow Diagram

```mermaid
flowchart LR
    subgraph Inbound
        HTTP[HTTP Request]
    end

    subgraph Gateway[api-gateway]
        Handler[HTTP Handler]
        Auth[Auth Middleware]
        Client[gRPC Client]
    end

    subgraph Service[svc-devices]
        GRPC[gRPC Handler]
        UseCase[Use Case]
        Repo[Repository]
    end

    subgraph Storage
        PG[(PostgreSQL)]
    end

    HTTP --> Handler
    Handler --> Auth
    Auth --> Client
    Client --> GRPC
    GRPC --> UseCase
    UseCase --> Repo
    Repo --> PG
```

### Component Diagram - api-gateway

```mermaid
graph TB
    subgraph Adapters[Adapters Layer]
        HTTP[HTTP Handlers]
        GRPC[gRPC Client]
    end

    subgraph UseCases[Use Cases Layer]
        Commands[Commands]
        Queries[Queries]
    end

    subgraph Domain[Domain Layer]
        Models[Device Model]
        Errors[Domain Errors]
    end

    subgraph Shared[Shared Components]
        Decorators[Decorators<br/>Logging/Metrics/Tracing]
        Backoff[Backoff Strategy]
    end

    HTTP --> Commands
    HTTP --> Queries
    Commands --> GRPC
    Queries --> GRPC
    Commands --> Models
    Queries --> Models
    Decorators --> Commands
    Decorators --> Queries
    GRPC --> Backoff
```

### State Diagram - Device

```mermaid
stateDiagram-v2
    [*] --> available: Create Device

    available --> in_use: Assign Device
    available --> inactive: Deactivate

    in_use --> available: Release Device
    in_use --> inactive: Deactivate

    inactive --> available: Reactivate

    available --> [*]: Delete
    inactive --> [*]: Delete

    note right of in_use
        Cannot delete
        Cannot update name/brand
    end note
```

### Deployment Diagram

```mermaid
graph TB
    subgraph Docker[Docker Compose]
        subgraph Network[internal network]
            Traefik[traefik:v3.6.5<br/>:80, :443, :8080]
            Swagger[swagger-ui:v5.31.0<br/>:8080]
            Gateway[api-gateway<br/>:8080]
            Devices[svc-devices<br/>:9090, :8081]
            DB[(postgres:17<br/>:5432)]
            Vault[(vault:1.21.1<br/>:8200)]
            VaultInit[vault-init<br/>initializer]
            KeyDB[(keydb:alpine<br/>:6379)]
        end
    end

    subgraph Volumes
        Certs[.certs/]
        Config[traefik/]
        VaultData[vault-storage/]
        KeyDBData[keydb-storage/]
    end

    Certs --> Traefik
    Config --> Traefik
    VaultData --> Vault
    KeyDBData --> KeyDB
    VaultInit -->|init secrets| Vault
```
