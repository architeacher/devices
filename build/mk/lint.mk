# Linting targets for golangci-lint
# Documentation: https://golangci-lint.run/

# golangci-lint Docker image version (use latest for most up-to-date linters)
GOLANGCI_LINT_VERSION ?= latest

# Docker run command for golangci-lint
# - Mount current directory as /app in container
# - Mount Go module cache for faster subsequent runs
# - Set working directory to /app
GOLANGCI_LINT_DOCKER := docker run --rm \
	-v "${CURDIR}:/app" \
	-v "${HOME}/go/pkg/mod:/go/pkg/mod:ro" \
	-w /app \
	golangci/golangci-lint:$(GOLANGCI_LINT_VERSION)

.PHONY: lint
lint: check-docker ## 🔍 Run golangci-lint on all Go code.
	$(call printMessage,"🔍  Linting Go code",$(INFO_CLR))
	$(GOLANGCI_LINT_DOCKER) golangci-lint run ./...

.PHONY: lint-fix
lint-fix: check-docker ## 🔧 Run golangci-lint with auto-fix enabled.
	$(call printMessage,"🔧  Linting Go code with auto-fix",$(INFO_CLR))
	$(GOLANGCI_LINT_DOCKER) golangci-lint run --fix ./...

.PHONY: lint-ci
lint-ci: check-docker ## 🤖 Run golangci-lint with JSON output for CI.
	$(call printMessage,"🤖  Linting Go code for CI",$(INFO_CLR))
	$(GOLANGCI_LINT_DOCKER) golangci-lint run --output.json.path=stdout ./...

.PHONY: lint-verbose
lint-verbose: check-docker ## 📝 Run golangci-lint with verbose output for debugging.
	$(call printMessage,"📝  Linting Go code (verbose)",$(INFO_CLR))
	$(GOLANGCI_LINT_DOCKER) golangci-lint run --verbose ./...
