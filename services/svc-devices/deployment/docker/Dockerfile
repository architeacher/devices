ARG     GO_VERSION

# Dev tooling stage (Air + Delve + grpc_health_probe)
FROM    golang:${GO_VERSION:-1.25}-alpine AS dev-tools

RUN     go install github.com/air-verse/air@v1.63.4 \
        && \
        go install github.com/go-delve/delve/cmd/dlv@v1.25.2 \
        && \
        go install github.com/fullstorydev/grpcurl/cmd/grpcurl@v1.9.3

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
        go build -o /out/svc-devices ./cmd/svc-devices

# Minimal runtime images per component
FROM       gcr.io/distroless/static-debian12:nonroot AS prod
WORKDIR    /app
COPY       --from=builder /out/svc-devices /app/svc-devices
COPY       --from=dev-tools /go/bin/grpc-health-probe /usr/local/bin/grpc_health_probe
EXPOSE     9090
ENTRYPOINT ["/app/svc-devices"]

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
        "${SRC_ARTIFACTS_DIR}/grpcurl" \
        "${ARTIFACTS_DIR}/"

RUN     ln -s "${ARTIFACTS_DIR}/grpc-health-probe" "${ARTIFACTS_DIR}/grpc_health_probe"

USER    "${APP_USER}"

EXPOSE  9090
