ARG     GO_VERSION

# Dev tooling stage (Air + Delve)
FROM    golang:${GO_VERSION:-1.25}-alpine AS dev-tools

RUN     go install github.com/air-verse/air@v1.63.4 \
        && \
        go install github.com/go-delve/delve/cmd/dlv@v1.25.2

# Build stage for production binaries
FROM    golang:${GO_VERSION:-1.25} AS builder

WORKDIR /src

COPY    go.mod go.sum ./
RUN     go mod download
COPY    . .

ARG     TARGETOS="linux"
ARG     TARGETARCH="amd64"

ENV     CGO_ENABLED=0 \
        GOOS="${TARGETOS}" \
        GOARCH="${TARGETARCH}"

RUN     --mount=type=cache,target=/root/.cache/go-build \
        --mount=type=cache,target=/go/pkg/mod \
        go build -o /out/api ./cmd/svc-api-gateway

# Minimal runtime images per component
FROM       gcr.io/distroless/static-debian12:nonroot AS prod
WORKDIR    /app
COPY       --from=builder /out/api /app/svc-api-gateway
ENTRYPOINT ["/app/svc-api-gateway"]

# Development image (default build target)
FROM    golang:${GO_VERSION:-1.25}-alpine AS dev

ARG     ARTIFACTS_DIR="/usr/local/bin"
ARG     SRC_ARTIFACTS_DIR="/go/bin"
ARG     PATH_PREFIX="./deployment/docker"
ARG     WORKDIR="/app"

ENV     CGO_ENABLED=0 \
        APP_ENV="development" \
        APP_GROUP="app" \
        APP_USER="app"

WORKDIR "${WORKDIR}"

RUN     apk --no-cache add -u \
            bash \
            build-base \
            ca-certificates \
            curl \
            git \
            jq \
            tzdata \
        && \
        update-ca-certificates \
        && \
        addgroup -S "${APP_GROUP}" \
        && \
        adduser -S -G "${APP_GROUP}" "${APP_USER}"

COPY    --from="dev-tools" \
        --chown="${APP_USER}:${APP_GROUP}" \
        "${SRC_ARTIFACTS_DIR}/air" \
        "${SRC_ARTIFACTS_DIR}/dlv" \
        "${ARTIFACTS_DIR}/"

USER    "${APP_USER}"

EXPOSE  8088
