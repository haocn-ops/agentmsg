#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"

: "${DATABASE_URL:=postgres://agentmsg:agentmsg@127.0.0.1:5432/agentmsg?sslmode=disable}"
: "${REDIS_URL:=redis://127.0.0.1:6379/0}"
: "${API_GATEWAY_PORT:=8080}"
: "${MESSAGE_ENGINE_PORT:=8081}"

api_log="$(mktemp)"
engine_log="$(mktemp)"
api_pid=""
engine_pid=""

cleanup() {
	if [[ -n "${api_pid}" ]]; then
		kill "${api_pid}" >/dev/null 2>&1 || true
		wait "${api_pid}" >/dev/null 2>&1 || true
	fi
	if [[ -n "${engine_pid}" ]]; then
		kill "${engine_pid}" >/dev/null 2>&1 || true
		wait "${engine_pid}" >/dev/null 2>&1 || true
	fi
	rm -f "${api_log}" "${engine_log}"
}

print_logs_and_fail() {
	local message="$1"

	echo "${message}" >&2
	echo "----- api-gateway log -----" >&2
	cat "${api_log}" >&2 || true
	echo "----- message-engine log -----" >&2
	cat "${engine_log}" >&2 || true
	exit 1
}

wait_for_http() {
	local url="$1"

	for _ in $(seq 1 60); do
		if curl -fsS "${url}" >/dev/null 2>&1; then
			return 0
		fi
		sleep 1
	done

	print_logs_and_fail "Timed out waiting for ${url}"
}

assert_http_contains() {
	local url="$1"
	local expected="$2"

	local body
	body="$(curl -fsS "${url}")" || print_logs_and_fail "Failed to query ${url}"
	if [[ "${body}" != *"${expected}"* ]]; then
		print_logs_and_fail "Response from ${url} did not contain ${expected}"
	fi
}

trap cleanup EXIT

AUTO_MIGRATE=true OTEL_ENABLED=false DATABASE_URL="${DATABASE_URL}" REDIS_URL="${REDIS_URL}" go run ./cmd/api-gateway >"${api_log}" 2>&1 &
api_pid="$!"

AUTO_MIGRATE=false OTEL_ENABLED=false DATABASE_URL="${DATABASE_URL}" REDIS_URL="${REDIS_URL}" go run ./cmd/message-engine >"${engine_log}" 2>&1 &
engine_pid="$!"

wait_for_http "http://127.0.0.1:${API_GATEWAY_PORT}/health"
wait_for_http "http://127.0.0.1:${API_GATEWAY_PORT}/ready"
assert_http_contains "http://127.0.0.1:9090/metrics" "agentmsg_"

wait_for_http "http://127.0.0.1:${MESSAGE_ENGINE_PORT}/health"
wait_for_http "http://127.0.0.1:${MESSAGE_ENGINE_PORT}/ready"
assert_http_contains "http://127.0.0.1:9091/metrics" "go_gc_duration_seconds"

echo "Smoke checks passed."
