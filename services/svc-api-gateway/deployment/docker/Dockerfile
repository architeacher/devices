ARG     GO_VERSION

# Dev tooling stage (Air + Delve)
FROM    golang:${GO_VERSION:-1.25}-alpine AS dev-tools

RUN     go install github.com/air-verse/air@v1.63.4 \
        && \
        go install github.com/go-delve/delve/cmd/dlv@v1.25.2

# Build stage for production binaries
FROM    golang:${GO_VERSION:-1.25} AS builder

ARG     WORKDIR="/src"
ARG     OUTPUT_DIR="/out"
ARG     CMD_PATH="./cmd/svc-api-gateway"
ARG     BINARY_NAME="svc-api-gateway"
ARG     TARGET_OS="linux"
ARG     TARGET_ARCH="amd64"

ENV     CGO_ENABLED=0 \
        GOOS="${TARGET_OS}" \
        GOARCH="${TARGET_ARCH}"

WORKDIR "${WORKDIR}"

COPY    go.mod go.sum ./
RUN     go mod download
COPY    . .

RUN     --mount=type=cache,target=/root/.cache/go-build \
        --mount=type=cache,target=/go/pkg/mod \
        go build -o "${OUTPUT_DIR}/${BINARY_NAME}" "${CMD_PATH}"

# Minimal runtime image for production
FROM    gcr.io/distroless/static-debian12:nonroot AS prod

ARG     WORKDIR="/app"
ARG     ARTIFACTS_DIR="/usr/local/bin"
ARG     BUILDER_OUTPUT_DIR="/out"
ARG     BINARY_NAME="svc-api-gateway"
ARG     APP_USER="nonroot"

WORKDIR "${WORKDIR}"

COPY    --from=builder "${BUILDER_OUTPUT_DIR}/${BINARY_NAME}" "${WORKDIR}/${BINARY_NAME}"

USER    "${APP_USER}"

EXPOSE  8088

ENTRYPOINT ["./${BINARY_NAME}"]

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
