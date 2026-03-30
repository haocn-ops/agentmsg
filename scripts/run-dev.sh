#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"

echo "Starting AgentMsg development environment..."

# Start dependencies
docker-compose -f deployments/docker/docker-compose.yml up -d postgres redis

# Wait for services
echo "Waiting for database..."
sleep 5

# Run migrations
echo "Running migrations..."
export DATABASE_URL="postgres://agentmsg:agentmsg@localhost:5432/agentmsg?sslmode=disable"
migrate -path internal/repository/migrations -database "$DATABASE_URL" up

# Start services
echo "Starting API Gateway..."
go run ./cmd/api-gateway &

echo "Starting Message Engine..."
go run ./cmd/message-engine &

echo "All services started!"
echo "API Gateway: http://localhost:8080"
echo "Message Engine: http://localhost:8081"
echo "Metrics: http://localhost:9090"

# Wait for any process
wait
