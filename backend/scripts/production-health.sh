#!/usr/bin/env bash
set -Eeuo pipefail

BASE_URL="${1:-https://calc-api.pdurl.cn}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-10}"

curl --fail --silent --show-error --max-time "$TIMEOUT_SECONDS" \
  "$BASE_URL/health" > /dev/null
curl --fail --silent --show-error --max-time "$TIMEOUT_SECONDS" \
  "$BASE_URL/ready" > /dev/null

echo "Production health check passed: $BASE_URL"
