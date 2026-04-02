# Multi-stage Dockerfile for Dagryn
# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/web

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# Copy frontend package files
COPY web/package.json web/pnpm-lock.yaml ./

# Install dependencies
RUN pnpm install --frozen-lockfile

# Copy frontend source
COPY web/ ./

# Build frontend (only vite build, skip the copy-dist step)
RUN pnpm exec vite build --config vite.config.mjs

# Stage 2: Build Go application
FROM golang:1.25-alpine AS go-builder

# Install build dependencies
RUN apk add --no-cache git make ca-certificates

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend from previous stage
COPY --from=frontend-builder /app/web/dist ./pkg/server/dashboard/dist

# Build the Go binary with version info
ARG GIT_VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
      -X github.com/mujhtech/dagryn/internal/version.Version=${GIT_VERSION} \
      -X github.com/mujhtech/dagryn/internal/version.Commit=${GIT_COMMIT} \
      -X github.com/mujhtech/dagryn/internal/version.BuildDate=${BUILD_DATE} \
      -X github.com/mujhtech/dagryn/pkg/api/handlers.Version=${GIT_VERSION}" \
    -o /bin/dagryn ./cmd/dagryn

# Stage 3: Final runtime image
FROM alpine:latest

# Install runtime dependencies. Build tools (node, go, etc.) are NOT needed
# here, but host-mode plugin installs require minimal runtime libs for
# downloaded musl binaries (e.g. Node.js from unofficial builds).
RUN apk add --no-cache ca-certificates tzdata git docker-cli libstdc++ libgcc

# Create non-root user
RUN addgroup -g 1000 dagryn && \
    adduser -D -u 1000 -G dagryn dagryn

WORKDIR /app

# Copy binary from builder
COPY --from=go-builder /bin/dagryn /usr/local/bin/dagryn

# Copy example config
COPY dagryn.server.example.toml /app/dagryn.server.toml

# Create necessary directories
RUN mkdir -p /app/data /app/cache /app/logs && \
    chown -R dagryn:dagryn /app

# Switch to non-root user
USER dagryn

# Expose ports
EXPOSE 9000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9000/health || exit 1

# Default command - run server
CMD ["dagryn", "server", "--config", "/app/dagryn.server.toml"]
