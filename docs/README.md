```bash
    ____            _
   / __ \___ _   __(_)_______  _____
  / / / / _ \ | / / / ___/ _ \/ ___/
 / /_/ /  __/ |/ / / /__/  __(__  )
/_____/\___/|___/_/\___/\___/____/

```

REST API for device management with hexagonal architecture and CQS.

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

## Documentation

- [Architecture](architecture.md) - ADRs and system diagrams
- [API Specification](openapi-spec/devices/v1/svc-api-gateway.yaml) - OpenAPI spec
