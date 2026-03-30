# Build and Development
###########################

.PHONY: help build run test test-coverage lint fmt deps clean docker-build docker-up docker-down smoke

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Binary names
API_GATEWAY_BINARY=api-gateway
MESSAGE_ENGINE_BINARY=message-engine

# Build directories
BUILD_DIR=./build
CMD_DIR=./cmd

# Docker
DOCKER_IMAGE=agentmsg
DOCKER_TAG=latest
DOCKER_REGISTRY=docker.io

# Help
help:
	@echo "AI Agent Messaging Platform - Build Commands"
	@echo ""
	@echo "Usage:"
	@echo "  make deps           Install dependencies"
	@echo "  make build         Build all binaries"
	@echo "  make run           Run all services"
	@echo "  make test          Run tests"
	@echo "  make test-coverage Run tests with coverage"
	@echo "  make lint          Run linter"
	@echo "  make fmt           Format code"
	@echo "  make docker-build  Build Docker images"
	@echo "  make docker-up     Start Docker Compose"
	@echo "  make docker-down   Stop Docker Compose"
	@echo "  make migrate       Run database migrations"
	@echo "  make smoke         Run local startup smoke checks"
	@echo "  make clean         Clean build artifacts"

# Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Build all binaries
build: build-api-gateway build-message-engine

build-api-gateway:
	@echo "Building API Gateway..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(API_GATEWAY_BINARY) $(CMD_DIR)/api-gateway

build-message-engine:
	@echo "Building Message Engine..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(MESSAGE_ENGINE_BINARY) $(CMD_DIR)/message-engine

# Run all services (for development)
run:
	@echo "Starting services..."
	./scripts/run-dev.sh

# Run tests
test:
	$(GOTEST) -v -race ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

smoke:
	./scripts/smoke.sh

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Format code
fmt:
	$(GOFMT) ./...
	$(GOVET) ./...

# Docker commands
docker-build:
	@echo "Building Docker images..."
	docker build -t $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)-api-gateway:$(DOCKER_TAG) -f deployments/docker/Dockerfile.api-gateway .
	docker build -t $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)-message-engine:$(DOCKER_TAG) -f deployments/docker/Dockerfile.message-engine .

docker-up:
	@echo "Starting Docker Compose..."
	docker compose -f deployments/docker/docker-compose.yml up -d

docker-down:
	@echo "Stopping Docker Compose..."
	docker compose -f deployments/docker/docker-compose.yml down

# Database migrations
migrate:
	@echo "Running migrations..."
	$(GOCMD) run ./cmd/migrate

migrate-down:
	@echo "Rolling back migrations..."
	@echo "Down migrations are not implemented for the embedded migration runner."

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	docker compose -f deployments/docker/docker-compose.yml down -v

# Development helpers
dev-api:
	$(GOCMD) run $(CMD_DIR)/api-gateway/main.go

dev-engine:
	$(GOCMD) run $(CMD_DIR)/message-engine/main.go

# Generate mocks for testing
generate-mocks:
	@echo "Generating mocks..."
	mockgen -source=internal/model/agent.go -destination=internal/mocks/agent_mock.go
	mockgen -source=internal/repository/agent_repository.go -destination=internal/mocks/agent_repository_mock.go

# Build for production
build-prod:
	@echo "Building production binaries..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -a -installsuffix cgo -o $(BUILD_DIR)/$(API_GATEWAY_BINARY) $(CMD_DIR)/api-gateway
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -a -installsuffix cgo -o $(BUILD_DIR)/$(MESSAGE_ENGINE_BINARY) $(CMD_DIR)/message-engine

# Kubernetes deployment
k8s-deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f deployments/k8s/

k8s-delete:
	@echo "Removing from Kubernetes..."
	kubectl delete -f deployments/k8s/

# Helm deployment
helm-install:
	@echo "Installing Helm chart..."
	helm install agentmsg deployments/helm/agentmsg

helm-upgrade:
	@echo "Upgrading Helm chart..."
	helm upgrade agentmsg deployments/helm/agentmsg

# Version info
version:
	@echo "Version: $$(git describe --tags --always --dirty)"

# Release
release: test lint build-prod docker-build
	@echo "Creating release..."
	git tag -a v$(VERSION) -m "Release v$(VERSION)"
	git push origin v$(VERSION)
