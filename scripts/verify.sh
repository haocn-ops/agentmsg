#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${REPO_ROOT}/deployments/docker/docker-compose.yml"

: "${DATABASE_URL:=postgres://agentmsg:agentmsg@127.0.0.1:5432/agentmsg?sslmode=disable}"
: "${REDIS_URL:=redis://127.0.0.1:6379/0}"
: "${VERIFY_MANAGE_DOCKER:=true}"
: "${VERIFY_CLEANUP_DOCKER:=true}"
: "${VERIFY_RUN_SMOKE:=true}"
: "${VERIFY_RUN_SDKS:=true}"
: "${VERIFY_GO_TEST_FLAGS:=}"

started_postgres="false"
started_redis="false"

cleanup() {
	if [[ "${VERIFY_MANAGE_DOCKER}" != "true" || "${VERIFY_CLEANUP_DOCKER}" != "true" ]]; then
		return
	fi

	local services=()
	if [[ "${started_postgres}" == "true" ]]; then
		services+=("postgres")
	fi
	if [[ "${started_redis}" == "true" ]]; then
		services+=("redis")
	fi
	if [[ "${#services[@]}" -gt 0 ]]; then
		echo "Stopping verify dependencies..."
		docker compose -f "${COMPOSE_FILE}" stop "${services[@]}" >/dev/null 2>&1 || true
	fi
}

require_command() {
	local cmd="$1"
	if ! command -v "${cmd}" >/dev/null 2>&1; then
		echo "Missing required command: ${cmd}" >&2
		exit 1
	fi
}

wait_for_tcp() {
	local host="$1"
	local port="$2"
	local label="$3"

	for _ in $(seq 1 60); do
		if nc -z "${host}" "${port}" >/dev/null 2>&1; then
			return 0
		fi
		sleep 1
	done

	echo "Timed out waiting for ${label} on ${host}:${port}" >&2
	exit 1
}

wait_for_health() {
	local container="$1"

	for _ in $(seq 1 60); do
		local status
		status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{if .State.Running}}running{{else}}stopped{{end}}{{end}}' "${container}" 2>/dev/null || true)"
		if [[ "${status}" == "healthy" || "${status}" == "running" ]]; then
			return 0
		fi
		sleep 1
	done

	echo "Timed out waiting for ${container} health" >&2
	exit 1
}

run_step() {
	local label="$1"
	shift
	echo
	echo "==> ${label}"
	"$@"
}

trap cleanup EXIT

cd "${REPO_ROOT}"

require_command go
require_command python3
require_command npm
require_command nc

if [[ "${VERIFY_MANAGE_DOCKER}" == "true" ]]; then
	require_command docker
	docker version >/dev/null

	if [[ "$(docker inspect -f '{{.State.Running}}' agentmsg-postgres 2>/dev/null || echo false)" != "true" ]]; then
		started_postgres="true"
	fi
	if [[ "$(docker inspect -f '{{.State.Running}}' agentmsg-redis 2>/dev/null || echo false)" != "true" ]]; then
		started_redis="true"
	fi

	run_step "Starting verify dependencies" docker compose -f "${COMPOSE_FILE}" up -d postgres redis
	wait_for_health "agentmsg-postgres"
	wait_for_health "agentmsg-redis"
fi

wait_for_tcp "127.0.0.1" "5432" "PostgreSQL"
wait_for_tcp "127.0.0.1" "6379" "Redis"

go_test_cmd=(go test ./...)
if [[ -n "${VERIFY_GO_TEST_FLAGS}" ]]; then
	# shellcheck disable=SC2206
	go_test_flags=(${VERIFY_GO_TEST_FLAGS})
	go_test_cmd=(go test "${go_test_flags[@]}" ./...)
fi

run_step "Running Go test suite" env DATABASE_URL="${DATABASE_URL}" REDIS_URL="${REDIS_URL}" "${go_test_cmd[@]}"

if [[ "${VERIFY_RUN_SDKS}" == "true" ]]; then
	run_step "Running Go SDK tests" bash -lc "cd '${REPO_ROOT}/sdk/go/agentmsg' && go test ./..."
	run_step "Running Python SDK tests" env PYTHONPATH="${REPO_ROOT}/sdk/python" python3 -m unittest discover -s "${REPO_ROOT}/sdk/python/tests"
	run_step "Running Node.js SDK tests" bash -lc "cd '${REPO_ROOT}/sdk/nodejs' && npm test"
fi

if [[ "${VERIFY_RUN_SMOKE}" == "true" ]]; then
	run_step "Running startup smoke checks" env DATABASE_URL="${DATABASE_URL}" REDIS_URL="${REDIS_URL}" "${REPO_ROOT}/scripts/smoke.sh"
fi

echo
echo "Verification passed."
