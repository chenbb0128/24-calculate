#!/usr/bin/env bash

# Shared implementation for project-specific, protected production .env updates.
# This file is sourced by a project deploy script; the caller provides log() and fail().

ZDZQ_ENV_CHANGED=0
ZDZQ_ENV_BACKUP_FILE=""
ZDZQ_ENV_TEMPLATE_TEMP=""
ZDZQ_ENV_RENDERED_TEMP=""

zdzq_env_validate_file() {
    local file="$1"
    local duplicate_keys
    local invalid_line_number

    [[ -f "$file" ]] || fail "Environment file not found: $file"
    if grep -q $'\r' "$file"; then
        fail "Environment file contains CR characters: $file"
    fi
    invalid_line_number="$(awk '!/^[A-Z][A-Z0-9_]*=.*/ && !/^#/ && !/^[[:space:]]*$/ { print NR; exit }' "$file")"
    [[ -z "$invalid_line_number" ]] \
        || fail "Environment file has an invalid line at $invalid_line_number: $file"
    duplicate_keys="$(sed -n 's/^\([A-Z][A-Z0-9_]*\)=.*/\1/p' "$file" | sort | uniq -d)"
    [[ -z "$duplicate_keys" ]] \
        || fail "Environment file contains duplicate keys: $duplicate_keys"
}

zdzq_env_array_contains() {
    local array_name="$1"
    local expected="$2"
    local item
    local -n array_ref="$array_name"

    for item in "${array_ref[@]}"; do
        [[ "$item" == "$expected" ]] && return 0
    done
    return 1
}

zdzq_env_cleanup() {
    if [[ -n "$ZDZQ_ENV_TEMPLATE_TEMP" ]]; then
        rm -f -- "$ZDZQ_ENV_TEMPLATE_TEMP"
        ZDZQ_ENV_TEMPLATE_TEMP=""
    fi
    if [[ -n "$ZDZQ_ENV_RENDERED_TEMP" ]]; then
        rm -f -- "$ZDZQ_ENV_RENDERED_TEMP"
        ZDZQ_ENV_RENDERED_TEMP=""
    fi
}

zdzq_env_restore() {
    local target="$1"
    local target_dir

    [[ "$ZDZQ_ENV_CHANGED" -eq 1 ]] || return 0
    [[ -n "$ZDZQ_ENV_BACKUP_FILE" && -s "$ZDZQ_ENV_BACKUP_FILE" ]] \
        || fail 'Protected production environment backup is unavailable for rollback'

    target_dir="$(dirname "$target")"
    log "Restoring protected production environment from $ZDZQ_ENV_BACKUP_FILE"
    ZDZQ_ENV_RENDERED_TEMP="$(mktemp "$target_dir/.env.rollback.XXXXXX")"
    install -m 0600 "$ZDZQ_ENV_BACKUP_FILE" "$ZDZQ_ENV_RENDERED_TEMP"
    chown root:root "$ZDZQ_ENV_RENDERED_TEMP"
    mv -f "$ZDZQ_ENV_RENDERED_TEMP" "$target"
    ZDZQ_ENV_RENDERED_TEMP=""
    ZDZQ_ENV_CHANGED=0
}

zdzq_env_install() {
    local target="$1"
    local payload="$2"
    local config_sha="$3"
    local backup_dir="$4"
    local secret_array_name="$5"
    local required_array_name="$6"
    local nonempty_array_name="$7"
    local assignment_key
    local backup_sha
    local line
    local marker_count
    local original_sha
    local required_key
    local secret_key
    local secret_line
    local target_dir
    local template_config_sha
    local timestamp
    local value
    local -n secret_keys="$secret_array_name"
    local -n required_keys="$required_array_name"

    [[ "$target" == /* && "$target" != "/" ]] || fail 'Protected environment target must be a safe absolute path'
    [[ "$backup_dir" == /* && "$backup_dir" != "/" ]] || fail 'Environment backup directory must be a safe absolute path'
    [[ "$config_sha" =~ ^[0-9a-f]{64}$ ]] || fail 'Expected a lowercase 64-character CONFIG_SHA'
    [[ -n "$payload" ]] || fail 'Managed production environment payload is required'
    [[ ${#payload} -le "${ZDZQ_ENV_PAYLOAD_MAX_CHARS:-262144}" ]] \
        || fail 'Managed production environment payload exceeds the configured limit'
    [[ -f "$target" ]] || fail "Protected production environment not found: $target"
    [[ "$(stat -c '%a' "$target")" == "600" ]] \
        || fail "Protected production environment must have mode 0600: $target"
    [[ "$(stat -c '%u:%g' "$target")" == "0:0" ]] \
        || fail "Protected production environment must be owned by root:root: $target"

    command -v base64 >/dev/null || fail 'base64 is not installed'
    command -v sha256sum >/dev/null || fail 'sha256sum is not installed'
    zdzq_env_validate_file "$target"

    ZDZQ_ENV_TEMPLATE_TEMP="$(mktemp /tmp/zdzq-env-prod.XXXXXX)"
    chmod 0600 "$ZDZQ_ENV_TEMPLATE_TEMP"
    if ! printf '%s' "$payload" | base64 --decode > "$ZDZQ_ENV_TEMPLATE_TEMP"; then
        fail 'Managed production environment payload is not valid base64'
    fi
    [[ -s "$ZDZQ_ENV_TEMPLATE_TEMP" ]] || fail 'Managed production environment payload is empty'
    zdzq_env_validate_file "$ZDZQ_ENV_TEMPLATE_TEMP"

    template_config_sha="$(grep -m1 '^ZDZQ_CONFIG_SHA=' "$ZDZQ_ENV_TEMPLATE_TEMP" | cut -d= -f2-)" \
        || fail 'Managed production environment is missing ZDZQ_CONFIG_SHA'
    [[ "$template_config_sha" == "$config_sha" ]] \
        || fail 'Managed production environment CONFIG_SHA does not match the requested configuration'

    for secret_key in "${secret_keys[@]}"; do
        grep -Fqx "${secret_key}=\${PROD_SECRET:${secret_key}}" "$ZDZQ_ENV_TEMPLATE_TEMP" \
            || fail "Managed production environment is missing protected marker: $secret_key"
    done
    marker_count="$(grep -Ec '=\$\{PROD_SECRET:[A-Z][A-Z0-9_]*\}$' "$ZDZQ_ENV_TEMPLATE_TEMP" || true)"
    [[ "$marker_count" -eq "${#secret_keys[@]}" ]] \
        || fail 'Managed production environment contains an undeclared or malformed protected marker'

    target_dir="$(dirname "$target")"
    ZDZQ_ENV_RENDERED_TEMP="$(mktemp "$target_dir/.env.new.XXXXXX")"
    while IFS= read -r line || [[ -n "$line" ]]; do
        if [[ "$line" =~ ^([A-Z][A-Z0-9_]*)=\$\{PROD_SECRET:([A-Z][A-Z0-9_]*)\}$ ]]; then
            assignment_key="${BASH_REMATCH[1]}"
            secret_key="${BASH_REMATCH[2]}"
            [[ "$assignment_key" == "$secret_key" ]] \
                || fail "Protected marker does not match assignment key: $assignment_key"
            secret_line="$(grep -m1 "^${secret_key}=" "$target")" \
                || fail "Protected production environment is missing required key: $secret_key"
            value="${secret_line#*=}"
            if zdzq_env_array_contains "$nonempty_array_name" "$secret_key"; then
                [[ -n "$value" ]] \
                    || fail "Protected production environment has an empty required key: $secret_key"
            fi
            printf '%s\n' "$secret_line" >> "$ZDZQ_ENV_RENDERED_TEMP"
        else
            printf '%s\n' "$line" >> "$ZDZQ_ENV_RENDERED_TEMP"
        fi
    done < "$ZDZQ_ENV_TEMPLATE_TEMP"

    zdzq_env_validate_file "$ZDZQ_ENV_RENDERED_TEMP"
    if grep -q '\${PROD_SECRET:' "$ZDZQ_ENV_RENDERED_TEMP"; then
        fail 'Rendered production environment still contains protected markers'
    fi
    for required_key in "${required_keys[@]}"; do
        grep -q "^${required_key}=" "$ZDZQ_ENV_RENDERED_TEMP" \
            || fail "Rendered production environment is missing required key: $required_key"
    done
    chmod 0600 "$ZDZQ_ENV_RENDERED_TEMP"
    chown root:root "$ZDZQ_ENV_RENDERED_TEMP"

    if cmp -s "$ZDZQ_ENV_RENDERED_TEMP" "$target"; then
        zdzq_env_cleanup
        log 'Managed production environment already matches requested configuration'
        return 0
    fi

    timestamp="$(date -u '+%Y%m%dT%H%M%SZ')"
    install -d -m 0700 "$backup_dir"
    ZDZQ_ENV_BACKUP_FILE="$(mktemp "$backup_dir/${timestamp}-${config_sha:0:12}.XXXXXX.env")"
    install -m 0600 "$target" "$ZDZQ_ENV_BACKUP_FILE"
    [[ -s "$ZDZQ_ENV_BACKUP_FILE" ]] || fail 'Protected production environment backup is empty'
    original_sha="$(sha256sum "$target" | awk '{print $1}')"
    backup_sha="$(sha256sum "$ZDZQ_ENV_BACKUP_FILE" | awk '{print $1}')"
    [[ "$original_sha" == "$backup_sha" ]] || fail 'Protected production environment backup hash mismatch'
    sha256sum "$ZDZQ_ENV_BACKUP_FILE" > "${ZDZQ_ENV_BACKUP_FILE}.sha256"
    chmod 0600 "${ZDZQ_ENV_BACKUP_FILE}.sha256"

    ZDZQ_ENV_CHANGED=1
    mv -f "$ZDZQ_ENV_RENDERED_TEMP" "$target"
    ZDZQ_ENV_RENDERED_TEMP=""
    rm -f -- "$ZDZQ_ENV_TEMPLATE_TEMP"
    ZDZQ_ENV_TEMPLATE_TEMP=""
    log "Installed managed production environment for CONFIG_SHA=$config_sha"
}
