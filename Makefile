.PHONY: build build-go frontend test install clean lint run swagger swagger-fmt server server-dev migrate proto-gen

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
SWAG=$(shell go env GOPATH)/bin/swag

# Version info
GIT_VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-X github.com/mujhtech/dagryn/internal/version.Version=$(GIT_VERSION) \
        -X github.com/mujhtech/dagryn/internal/version.Commit=$(GIT_COMMIT) \
        -X github.com/mujhtech/dagryn/internal/version.BuildDate=$(BUILD_DATE) \
        -X github.com/mujhtech/dagryn/pkg/api/handlers.Version=$(GIT_VERSION)

# Build the frontend
frontend:
	cd web && pnpm build

# Build the binary
build: frontend
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/dagryn

# Build Go binary only (without frontend)
build-go:
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/dagryn

# Build with version info
build-release:
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS) -s -w" -o $(BUILD_DIR)/$(BINARY) ./cmd/dagryn

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
	$(SWAG) init -g pkg/server/server.go -o docs --parseDependency --parseInternal

# Format swagger comments
swagger-fmt:
	$(SWAG) fmt -g pkg/server/server.go

# Install swagger CLI tool
swagger-install:
	$(GOCMD) install github.com/swaggo/swag/cmd/swag@latest

# Start the server (development)
server: build
	./$(BUILD_DIR)/$(BINARY) server $(ARGS)

# Start the server with .env and dagryn.server.toml config
server-dev: build
	./$(BUILD_DIR)/$(BINARY) server --config dagryn.server.toml $(ARGS)


worker-dev:
	./$(BUILD_DIR)/$(BINARY) worker --config dagryn.server.toml $(ARGS)

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

# Generate protobuf code (requires buf, protoc-gen-go, protoc-gen-go-grpc)
# Install: brew install buf && go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
proto-gen:
	PATH=$(PATH):$(shell go env GOPATH)/bin buf generate

# Docker build
docker-build:
	docker build -t dagryn:latest .

# Docker run
docker-run:
	docker run -p 9000:9000 dagryn:latest
