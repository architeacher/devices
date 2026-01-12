ENV_FILE := .envrc
DOT_ENV := .env
CERTS_DIR := ".certs"
SERVICES_DIR := services

PROJECT_NAME := "devices"
VERSION ?= "v1"

MOCKS_DIR := services/svc-devices/internal/mocks

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

.PHONY: generate-api
generate-api: ## 🤖 Generate API specs from OpenAPI definition.
	$(call printMessage,"🤖  Generating API specs",$(INFO_CLR))
	docker run --rm \
		-v "${CURDIR}/docs/openapi-spec":/spec \
		-w "/spec" \
		redocly/cli:2.13.0 bundle \
	    "devices/${VERSION}/svc-api-gateway.yaml" \
		-d \
		--output "devices/${VERSION}/public/svc-api-gateway-swagger.json" \
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
test-integration: ## 🔗 Run integration tests with race detection (requires Docker).
	$(call printMessage,"🔗  Running integration tests",$(INFO_CLR))
	for dir in ${SERVICES_DIR}/*/; do \
		if [ -f "$${dir}go.mod" ] && [ -d "$${dir}itest" ]; then \
			echo "Testing $${dir}..."; \
			(cd "$${dir}" && go test -v -race -tags=integration ./itest/...) || exit 1; \
		fi \
	done
