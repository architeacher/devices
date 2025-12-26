#!/usr/bin/env bash

set -e

SERVICES="${VAULT_SERVICES:-svc-devices,svc-api-gateway}"
SECRET_ENGINE_PATH="${VAULT_SECRET_ENGINE_PATH:-apps}"
SERVICES_CONFIG_DIR="${SERVICES_CONFIG_DIR:-/app/services}"
MAX_WAIT_ATTEMPTS=30
WAIT_INTERVAL=2

exit_status=0

log_info() {
    echo "[INFO] ${1}"
}

log_warning() {
    echo "[WARNING] ${1}"
}

log_error() {
    echo "[ERROR] ${1}" >&2
}

abort() {
    log_error "${1}"
    exit 1
}

load_configuration() {
    log_info "Loading infrastructure configuration..."

    if [ -f "/app/.envrc" ]; then
        original_vault_addr="${VAULT_ADDR}"

        set -a
        # shellcheck source=/dev/null
        . /app/.envrc
        set +a

        if [ -n "${original_vault_addr}" ]; then
            export VAULT_ADDR="${original_vault_addr}"
        fi

        log_info "Loaded infrastructure configuration from /app/.envrc"

        return 0
    fi

    log_warning "Infrastructure .envrc file not found, using environment defaults"

    return 1
}

wait_for_vault() {
    log_info "Waiting for Vault to be ready..."

    attempt=0

    while [ "${attempt}" -lt "${MAX_WAIT_ATTEMPTS}" ]; do
        if wget --quiet --spider "${VAULT_ADDR}/v1/sys/health" 2>/dev/null; then
            log_info "Vault is ready!"

            return 0
        fi

        log_info "Vault is unavailable - sleeping (attempt ${attempt}/${MAX_WAIT_ATTEMPTS})"
        sleep "${WAIT_INTERVAL}"
        attempt=$((attempt + 1))
    done

    abort "Vault failed to become ready after ${MAX_WAIT_ATTEMPTS} attempts"
}

is_vault_initialized() {
    first_service=$(echo "${SERVICES}" | cut -d',' -f1)

    if vault kv get "${SECRET_ENGINE_PATH}/${first_service}" >/dev/null 2>&1; then
        log_info "Vault is already configured"

        return 0
    fi

    return 1
}

setup_vault_authentication() {
    log_info "Setting up Vault authentication..."

    if [ -z "${VAULT_ROOT_TOKEN}" ]; then
        abort "VAULT_ROOT_TOKEN is not set"
    fi

    export VAULT_TOKEN="${VAULT_ROOT_TOKEN}"
}

enable_secret_engines() {
    log_info "Enabling secret engines..."

    log_info "Enabling KV secrets engine at '${SECRET_ENGINE_PATH}' path..."
    if ! vault secrets enable -path="${SECRET_ENGINE_PATH}" -version=2 kv 2>/dev/null; then
        log_info "Engine already exists or failed to enable - continuing..."
    fi
}

extract_env_variables() {
    config_file="${1}"
    temp_file="${2}"

    grep -E "^[A-Z_]+=.+" "${config_file}" > "${temp_file}" || true

    log_info "Found $(wc -l < "${temp_file}" | tr -d ' ') environment variables in ${config_file}"
}

build_vault_args() {
    input_file="${1}"
    output_file="${2}"
    count=0

    : > "${output_file}"

    while IFS='=' read -r key value; do
        if [ -z "${key}" ]; then
            continue
        fi

        value=$(echo "${value}" | sed 's/^"\(.*\)"$/\1/' | sed "s/^'\(.*\)'$/\1/")
        printf '%s=%s\n' "${key}" "${value}" >> "${output_file}"
        count=$((count + 1))
    done < "${input_file}"

    echo "${count}"
}

store_secrets_for_service() {
    service="${1}"
    vault_args_file="${2}"
    secret_path="${SECRET_ENGINE_PATH}/${service}"

    log_info "Storing secrets under '${secret_path}'..."

    if ! xargs vault kv put "${secret_path}" < "${vault_args_file}"; then
        abort "Failed to store secrets for service '${service}'"
    fi

    log_info "Secrets stored successfully for service '${service}'"
}

trim() {
    echo "${1}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//'
}

foreach_service() {
    callback="${1}"
    shift

    old_ifs="${IFS}"
    IFS=','

    for service in ${SERVICES}; do
        service=$(trim "${service}")
        [ -z "${service}" ] && continue
        "${callback}" "${service}" "$@"
    done

    IFS="${old_ifs}"
}

store_secrets_for_single_service() {
    service="${1}"
    config_file="${SERVICES_CONFIG_DIR}/${service}/.envrc"

    if [ ! -f "${config_file}" ]; then
        log_warning "Config file not found for service '${service}': ${config_file}"

        return 0
    fi

    log_info "Processing secrets for service '${service}' from ${config_file}..."

    temp_file="/tmp/vault_vars_${service}"
    vault_args_file="/tmp/vault_args_${service}"

    extract_env_variables "${config_file}" "${temp_file}"

    count=$(build_vault_args "${temp_file}" "${vault_args_file}")
    rm -f "${temp_file}"

    if [ "${count}" -eq 0 ]; then
        rm -f "${vault_args_file}"
        log_warning "No valid environment variables found for service '${service}'"

        return 0
    fi

    log_info "Storing ${count} variables for service '${service}'..."
    store_secrets_for_service "${service}" "${vault_args_file}"

    rm -f "${vault_args_file}"
}

store_application_secrets() {
    log_info "Storing application secrets from per-service configs..."

    foreach_service store_secrets_for_single_service
}

create_vault_policy() {
    service="${1}"
    policy_name="${service}-policy"
    secret_path="${SECRET_ENGINE_PATH}/data/${service}"

    log_info "Creating policy '${policy_name}' for service '${service}'..."

    if ! vault policy write "${policy_name}" - <<EOF
# Read application secrets for ${service}
path "${secret_path}" {
  capabilities = ["read"]
}

# Allow listing secrets
path "${SECRET_ENGINE_PATH}/metadata/*" {
  capabilities = ["list"]
}
EOF
    then
        abort "Failed to create policy for service '${service}'"
    fi

    log_info "Policy '${policy_name}' created successfully"
}

create_all_policies() {
    log_info "Creating policies for all services..."

    foreach_service create_vault_policy
}

setup_approle_for_service() {
    service="${1}"
    policy_name="${service}-policy"
    role_name="${service}"

    log_info "Creating AppRole '${role_name}' for service '${service}'..."

    if ! vault write "auth/approle/role/${role_name}" \
        token_policies="${policy_name}" \
        token_ttl=1h \
        token_max_ttl=4h; then
        abort "Failed to create AppRole for service '${service}'"
    fi

    log_info "AppRole '${role_name}' created successfully"
}

setup_approle_authentication() {
    log_info "Setting up AppRole authentication..."

    log_info "Enabling AppRole authentication..."
    if ! vault auth enable approle 2>/dev/null; then
        log_info "AppRole already enabled - continuing..."
    fi

    foreach_service setup_approle_for_service
}

store_approle_credentials() {
    service="${1}"
    role_name="${service}"
    secret_path="${SECRET_ENGINE_PATH}/${service}"

    log_info "Storing AppRole credentials for service '${service}'..."

    role_id=$(vault read -field=role_id "auth/approle/role/${role_name}/role-id")
    if [ -z "${role_id}" ]; then
        abort "Failed to get role ID for service '${service}'"
    fi

    secret_id=$(vault write -force -field=secret_id "auth/approle/role/${role_name}/secret-id")
    if [ -z "${secret_id}" ]; then
        abort "Failed to get secret ID for service '${service}'"
    fi

    if ! vault kv patch "${secret_path}" \
        VAULT_ROLE_ID="${role_id}" \
        VAULT_SECRET_ID="${secret_id}"; then
        abort "Failed to store AppRole credentials for service '${service}'"
    fi

    log_info "AppRole credentials stored successfully for service '${service}'"
}

store_all_approle_credentials() {
    log_info "Storing AppRole credentials for all services..."

    foreach_service store_approle_credentials
}

initialize_vault() {
    log_info "Configuring Vault for services: ${SERVICES}"

    enable_secret_engines
    store_application_secrets
    create_all_policies
    setup_approle_authentication
    store_all_approle_credentials

    log_info "Vault initialization completed successfully!"
}

main() {
    trap 'exit ${exit_status}' EXIT

    load_configuration
    wait_for_vault
    setup_vault_authentication

    if is_vault_initialized; then
        exit 0
    fi

    initialize_vault
}

main "${@}"
