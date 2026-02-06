.PHONY: build build-go frontend test install clean lint run swagger swagger-fmt server server-dev migrate

# Binary name
BINARY=dagryn

# Build directory
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Swagger
SWAG=swag

# Build the frontend
frontend:
	cd web && npm run build

# Build the binary
build: frontend
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY) ./cmd/dagryn

# Build Go binary only (without frontend)
build-go:
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY) ./cmd/dagryn

# Build with version info
build-release:
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags "-X github.com/mujhtech/dagryn/internal/server/handlers.Version=$(shell git describe --tags --always --dirty)" -o $(BUILD_DIR)/$(BINARY) ./cmd/dagryn

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Install the binary
install:
	$(GOCMD) install ./cmd/dagryn

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -rf .dagryn/
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Run the dagryn command (for testing)
run: build
	./$(BUILD_DIR)/$(BINARY) $(ARGS)

# Format code
fmt:
	$(GOCMD) fmt ./...

# Vet code
vet:
	$(GOCMD) vet ./...

# Generate swagger documentation
swagger:
	$(SWAG) init -g internal/server/server.go -o docs --parseDependency --parseInternal

# Format swagger comments
swagger-fmt:
	$(SWAG) fmt -g internal/server/server.go

# Install swagger CLI tool
swagger-install:
	$(GOCMD) install github.com/swaggo/swag/cmd/swag@latest

# Start the server (development)
server: build
	./$(BUILD_DIR)/$(BINARY) server $(ARGS)

# Start the server with .env and dagryn.server.toml config
server-dev: build
	@set -a && source .env && set +a && ./$(BUILD_DIR)/$(BINARY) server --config dagryn.server.toml $(ARGS)


worker-dev:
	@set -a && source .env && set +a && ./$(BUILD_DIR)/$(BINARY) worker --config dagryn.server.toml $(ARGS)

# Run database migrations
migrate: build
	./$(BUILD_DIR)/$(BINARY) migrate up $(ARGS)

# Show migration status
migrate-status: build
	./$(BUILD_DIR)/$(BINARY) migrate status $(ARGS)

# Rollback last migration
migrate-down: build
	./$(BUILD_DIR)/$(BINARY) migrate down $(ARGS)

# Development: run with hot reload (requires air)
dev:
	air -c .air.toml

# Lint code (requires golangci-lint)
lint:
	golangci-lint run ./...

# All checks before commit
check: fmt vet lint test

# Docker build
docker-build:
	docker build -t dagryn:latest .

# Docker run
docker-run:
	docker run -p 9000:9000 dagryn:latest
