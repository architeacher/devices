ENV_FILE := .envrc
DOT_ENV := .env
CERTS_DIR := ".certs"
SERVICES_DIR := services

PROJECT_NAME := "devices"
VERSION ?= "v1"

MOCKS_DIR := services/svc-devices/internal/mocks

CACHE_HOST ?= keydb
CACHE_PORT ?= 6379
CACHE_PASSWORD ?= ""

.PHONY: $(ENV_FILE) $(DOT_ENV)
$(ENV_FILE) $(DOT_ENV):
	cat .envrc.dist | tee "$(ENV_FILE)" "$(DOT_ENV)" > /dev/null

.PHONY: init-services
init-services: ## Generate per-service .envrc files from templates.
	$(call printMessage,"🔧  Generating per-service .envrc files",$(INFO_CLR))
	@for dir in $(SERVICES_DIR)/*/; do \
		if [ -f "$${dir}.envrc.dist" ]; then \
			cp "$${dir}.envrc.dist" "$${dir}.envrc"; \
			echo "Created $${dir}.envrc"; \
		fi \
	done

$(CERTS_DIR):
	mkdir -p "${CERTS_DIR}"

.PHONY: set-hosts
set-hosts: ## Update local hosts.
	$(call printMessage,"🤖  Updating local hosts",$(INFO_CLR))
	echo "\n# Devices Hosts\n\
====================\n\
127.0.0.1 api.${PROJECT_NAME}.dev docs.${PROJECT_NAME}.dev vault.${PROJECT_NAME}.dev" | sudo tee -a /etc/hosts

.PHONY: init
init: $(ENV_FILE) init-services set-hosts certify
	$(MAKE) generate-api

.PHONY: check-docker
check-docker: ## 🐳 Check if Docker daemon is running.
	@docker info > /dev/null 2>&1 || \
		{ printf "${ERROR_CLR}${MSG_PRFX} Docker is not running. Please start Docker Desktop.${MSG_SFX}${NO_CLR}\n"; exit 1; }

.PHONY: start
start: ## 🐳 Start the Docker containers.
	$(call printMessage,"🏁  Starting containers",$(INFO_CLR))
	docker compose \
			--profile development \
			up \
			-d \
    		--force-recreate

.PHONY: restart
restart: ## 🐳 Restart the Docker containers.
	$(call printMessage,"♻️  Restarting containers",$(INFO_CLR))
	docker compose \
			--profile development \
			restart

.PHONY: destroy
destroy: ## 🐳 Destroy Docker containers.
	$(call printMessage,"💣  Destroying containers",$(INFO_CLR))
	docker compose \
			down --remove-orphans

.PHONY: study
study: $(CERTS_DIR) ## 👨‍🎓 Studying hard and preparing for certification.
	$(call printMessage,"📖  Studying for the certification",$(INFO_CLR))
ifeq (, $(shell which "mkcert"))
 $(error "Command mkcert not found in $$PATH, please install https://github.com/FiloSottile/mkcert#installation")
endif
	mkcert -install

.PHONY: certify
certify: study ## 📜 Certify .localhost and .dev TLDs.
	$(call printMessage,"📚  Preparing for the certification",$(INFO_CLR))
	mkcert -cert-file "${CERTS_DIR}/star-${PROJECT_NAME}-dev.crt" \
		-key-file "${CERTS_DIR}/star-${PROJECT_NAME}-dev.key" \
		"${PROJECT_NAME}.dev" "*.${PROJECT_NAME}.dev"
	cp "$$(mkcert -CAROOT)/rootCA.pem" "${CERTS_DIR}/"

.PHONY: lint-api
lint-api: ## 🔍 Lint and validate OpenAPI specification.
	$(call printMessage,"🔍  Linting OpenAPI specification",$(INFO_CLR))
	docker run --rm \
		-v "${CURDIR}/docs/contracts/openapi":/spec \
		-w "/spec" \
		redocly/cli:latest lint \
		"devices/${VERSION}/specs.yaml" \
		--config .redocly.yaml

.PHONY: generate-api
generate-api: lint-api ## 🤖 Generate API specs from OpenAPI definition.
	$(call printMessage,"🤖  Generating API specs",$(INFO_CLR))
	docker run --rm \
		-v "${CURDIR}/docs/contracts/openapi":/spec \
		-w "/spec" \
		redocly/cli:latest bundle \
		"devices/${VERSION}/specs.yaml" \
		--output "devices/${VERSION}/public/specs-swagger.json" \
		--ext json \
		--config .redocly.yaml \
	&& \
	cd services/svc-api-gateway/internal/tools && go generate .

.PHONY: create-migration
create-migration: ## 🗂️ Creates migration files based on a passed argument "migration_name".
	$(call printMessage,"🗃️  Creating migration",$(INFO_CLR))
	docker compose run --name migrate-this --rm -it migrate create -ext sql -dir migrations "${migration_name}"

$(MOCKS_DIR):
	$(call printMessage,"🎭  Generating mocks",$(INFO_CLR))
	for dir in ${SERVICES_DIR}/*/; do \
		if [ -f "$${dir}go.mod" ]; then \
			echo "Generating mocks for $${dir}..."; \
			(cd "$${dir}" && GOFLAGS="-mod=mod" go generate ./...) || exit 1; \
		fi \
	done

.PHONY: generate-mocks
generate-mocks: $(MOCKS_DIR) ## 🎭 Generate test mocks from interfaces (only if needed).

.PHONY: generate-mocks-force
generate-mocks-force: ## 🎭 Force regenerate test mocks from interfaces.
	$(call printMessage,"🎭  Force regenerating mocks",$(INFO_CLR))
	rm -rf "${MOCKS_DIR}"
	$(MAKE) generate-mocks

.PHONY: test-unit
test-unit: generate-mocks ## 🧪 Run unit tests with race detection.
	$(call printMessage,"🧪  Running unit tests",$(INFO_CLR))
	for dir in ${SERVICES_DIR}/*/; do \
		if [ -f "$${dir}go.mod" ]; then \
			echo "Testing $${dir}..."; \
			(cd "$${dir}" && go test -v -race ./...) || exit 1; \
		fi \
	done

.PHONY: test-integration
test-integration: check-docker ## 🔗 Run integration tests with race detection (requires Docker).
	$(call printMessage,"🔗  Running integration tests",$(INFO_CLR))
	for dir in ${SERVICES_DIR}/*/; do \
		if [ -f "$${dir}go.mod" ] && [ -d "$${dir}itest" ]; then \
			echo "Testing $${dir}..."; \
			(cd "$${dir}" && go test -v -race -tags=integration ./itest/...) || exit 1; \
		fi \
	done

.PHONY: cache-purge
cache-purge: ## 🗑️ Purge all device caches from KeyDB.
	$(call printMessage,"🗑️  Purging all device caches",$(INFO_CLR))
	docker compose exec svc-api-gateway /artifacts/redis-cli -h "${CACHE_HOST}" -p "${CACHE_PORT}" \
		$(if ${CACHE_PASSWORD},-a "${CACHE_PASSWORD}" --no-auth-warning,) \
		EVAL "local cursor = '0'; local count = 0; repeat local result = redis.call('SCAN', cursor, 'MATCH', 'device*', 'COUNT', 1000); cursor = result[1]; for _,k in ipairs(result[2]) do redis.call('DEL', k); count = count + 1; end; until cursor == '0'; return count" 0

.PHONY: cache-purge-lists
cache-purge-lists: ## 🗑️ Purge device list caches only from KeyDB.
	$(call printMessage,"🗑️  Purging device list caches",$(INFO_CLR))
	docker compose exec svc-api-gateway /artifacts/redis-cli -h "${CACHE_HOST}" -p "${CACHE_PORT}" \
		$(if ${CACHE_PASSWORD},-a "${CACHE_PASSWORD}" --no-auth-warning,) \
		EVAL "local cursor = '0'; local count = 0; repeat local result = redis.call('SCAN', cursor, 'MATCH', 'devices:list*', 'COUNT', 1000); cursor = result[1]; for _,k in ipairs(result[2]) do redis.call('DEL', k); count = count + 1; end; until cursor == '0'; return count" 0

.PHONY: cache-purge-device
cache-purge-device: ## 🗑️ Purge a specific device cache by ID (usage: make cache-purge-device ID=<uuid>).
	$(call printMessage,"🗑️  Purging device cache for ID: ${ID}",$(INFO_CLR))
ifndef ID
	$(error ID is required. Usage: make cache-purge-device ID=<uuid>)
endif
	docker compose exec svc-api-gateway /artifacts/redis-cli -h "${CACHE_HOST}" -p "${CACHE_PORT}" \
		$(if ${CACHE_PASSWORD},-a "${CACHE_PASSWORD}" --no-auth-warning,) \
		DEL "device:v1:${ID}"

.PHONY: cache-stats
cache-stats: ## 📊 Show cache statistics from KeyDB.
	$(call printMessage,"📊  Cache statistics",$(INFO_CLR))
	docker compose exec svc-api-gateway /artifacts/redis-cli -h "${CACHE_HOST}" -p "${CACHE_PORT}" \
		$(if ${CACHE_PASSWORD},-a "${CACHE_PASSWORD}" --no-auth-warning,) \
		INFO memory | grep -E "used_memory_human|maxmemory_human|expired_keys|evicted_keys"

.PHONY: cache-keys
cache-keys: ## 🔑 List all device cache keys (for debugging).
	$(call printMessage,"🔑  Listing device cache keys",$(INFO_CLR))
	docker compose exec svc-api-gateway /artifacts/redis-cli -h "${CACHE_HOST}" -p "${CACHE_PORT}" \
		$(if ${CACHE_PASSWORD},-a "${CACHE_PASSWORD}" --no-auth-warning,) \
		EVAL "local cursor = '0'; local keys = {}; repeat local result = redis.call('SCAN', cursor, 'MATCH', 'device*', 'COUNT', 100); cursor = result[1]; for _,k in ipairs(result[2]) do table.insert(keys, k); end; until cursor == '0'; return keys" 0
