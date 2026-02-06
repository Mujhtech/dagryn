# AGENT_PLAN_V2.md

This document provides a comprehensive agentic plan for implementing three major features:

1. **Remote Cache Sharing** - Share task cache across machines using Bazel Remote APIs
2. **GitHub Actions Integration** - Run Dagryn workflows as GitHub Actions
3. **Dagryn Plugin Registry** - A first-party plugin system where users can build, publish, and share plugins

---

## Overview & Dependencies

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           DAGRYN V2 ARCHITECTURE                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────────────┐  │
│  │  CLI Client  │    │  GitHub App  │    │  Web Dashboard               │  │
│  └──────┬───────┘    └──────┬───────┘    └──────────────┬───────────────┘  │
│         │                   │                           │                   │
│         └───────────────────┼───────────────────────────┘                   │
│                             │                                               │
│                             ▼                                               │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                         DAGRYN SERVER                                │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────────┐  │  │
│  │  │ Run Engine │  │ Cache API  │  │ Plugin API │  │ Webhook Handler│  │  │
│  │  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘  └───────┬────────┘  │  │
│  └────────┼───────────────┼───────────────┼─────────────────┼───────────┘  │
│           │               │               │                 │               │
│           ▼               ▼               ▼                 ▼               │
│  ┌────────────────┐ ┌───────────┐ ┌─────────────────┐ ┌───────────────┐    │
│  │ Job Queue      │ │ Remote    │ │ Plugin Registry │ │ Git Providers │    │
│  │ (Redis/NATS)   │ │ Cache     │ │ (S3 + DB)       │ │ (GH/GL/BB)    │    │
│  └────────────────┘ │ (CAS+AC)  │ └─────────────────┘ └───────────────┘    │
│                     └───────────┘                                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

# Part 1: Remote Cache Sharing

## 1.1 Goal

Enable teams to share task cache across machines and CI runs, dramatically reducing build times for unchanged tasks.

## 1.2 Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     REMOTE CACHE FLOW                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Developer A                     Developer B                    │
│  ┌─────────┐                     ┌─────────┐                   │
│  │ dagryn  │                     │ dagryn  │                   │
│  │  run    │                     │  run    │                   │
│  └────┬────┘                     └────┬────┘                   │
│       │                               │                         │
│       ▼                               ▼                         │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              REMOTE CACHE SERVICE                        │   │
│  │  ┌─────────────────┐    ┌─────────────────────────────┐ │   │
│  │  │  Action Cache   │    │  Content Addressable Store  │ │   │
│  │  │  (ActionResult) │    │  (Blobs: inputs, outputs)   │ │   │
│  │  └─────────────────┘    └─────────────────────────────┘ │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│                               ▼                                 │
│                    ┌─────────────────────┐                     │
│                    │  Object Storage     │                     │
│                    │  (S3/GCS/MinIO)     │                     │
│                    └─────────────────────┘                     │
└─────────────────────────────────────────────────────────────────┘
```

## 1.3 Implementation Phases

### Phase 1.3.1: Remote Cache Protocol (Bazel Remote APIs)

**Goal:** Implement a gRPC client for Bazel Remote Execution API (cache only).

**Files to create:**
```
internal/cache/remote/
├── client.go          # gRPC client wrapper
├── digest.go          # Digest calculation (SHA256 + size)
├── action.go          # Action encoding for cache keys
├── cas.go             # Content Addressable Storage operations
├── actioncache.go     # Action Cache operations
└── config.go          # Remote cache configuration
```

**Key interfaces:**
```go
// RemoteCache implements the cache backend for remote storage
type RemoteCache interface {
    // Check if action result exists in remote cache
    Check(ctx context.Context, actionDigest *Digest) (*ActionResult, error)

    // Upload action result and outputs to remote cache
    Upload(ctx context.Context, actionDigest *Digest, result *ActionResult, outputs []OutputFile) error

    // Download outputs from remote cache
    Download(ctx context.Context, digests []*Digest, destDir string) error

    // GetCapabilities returns server capabilities
    GetCapabilities(ctx context.Context) (*Capabilities, error)
}

// Digest represents a content digest (SHA256 hash + size)
type Digest struct {
    Hash      string
    SizeBytes int64
}
```

**Tasks:**
- [ ] Implement Digest calculation matching Bazel format
- [ ] Implement Action encoding (deterministic serialization of task definition)
- [ ] Implement CAS client (BatchUpdateBlobs, BatchReadBlobs, FindMissingBlobs)
- [ ] Implement ActionCache client (GetActionResult, UpdateActionResult)
- [ ] Add gRPC connection management with retry and timeout
- [ ] Add authentication support (API key, mTLS, OAuth)

### Phase 1.3.2: Cache Backend Abstraction

**Goal:** Abstract cache backend so local and remote can be used interchangeably.

**Files to modify/create:**
```
internal/cache/
├── backend.go         # Backend interface (NEW)
├── local.go           # Local filesystem backend (refactor from store.go)
├── remote/            # Remote backend (from Phase 1.3.1)
├── hybrid.go          # Hybrid backend (local + remote) (NEW)
└── cache.go           # Updated to use backend interface
```

**Backend interface:**
```go
type Backend interface {
    // Check returns cache hit status and metadata
    Check(ctx context.Context, key string) (hit bool, metadata *CacheMetadata, err error)

    // Restore retrieves cached outputs to the workspace
    Restore(ctx context.Context, key string, destDir string) error

    // Save stores task outputs to cache
    Save(ctx context.Context, key string, outputs []string, metadata *CacheMetadata) error
}

type HybridBackend struct {
    local  Backend
    remote Backend
    config HybridConfig
}

type HybridConfig struct {
    // Strategy: "remote-first", "local-first", "write-through"
    Strategy string
    // FallbackOnError: continue with local if remote fails
    FallbackOnError bool
}
```

**Tasks:**
- [ ] Define Backend interface
- [ ] Refactor local cache to implement Backend
- [ ] Implement remote backend using RemoteCache
- [ ] Implement HybridBackend with configurable strategies
- [ ] Update scheduler to use Backend abstraction

### Phase 1.3.3: Configuration & CLI

**Goal:** Allow users to configure remote cache via config and CLI.

**Configuration (dagryn.toml):**
```toml
[cache]
# Local cache settings
local = true
dir = ".dagryn/cache"

[cache.remote]
enabled = true
endpoint = "grpc://cache.example.com:9092"
# Or use Dagryn Cloud cache
cloud = true

# Authentication
[cache.remote.auth]
type = "api-key"  # or "mtls", "oauth"
api_key_env = "DAGRYN_CACHE_API_KEY"

# Or mTLS
# type = "mtls"
# cert_file = "/path/to/cert.pem"
# key_file = "/path/to/key.pem"
# ca_file = "/path/to/ca.pem"
```

**CLI flags:**
```bash
dagryn run --remote-cache             # Enable remote cache
dagryn run --no-remote-cache          # Disable remote cache
dagryn run --cache-endpoint URL       # Override cache endpoint
dagryn cache status                   # Show cache statistics
dagryn cache push                     # Push local cache to remote
dagryn cache pull                     # Pull from remote to local
```

**Tasks:**
- [ ] Add cache configuration to config schema
- [ ] Add CLI flags for cache control
- [ ] Add `dagryn cache` subcommands
- [ ] Add cache statistics and reporting

### Phase 1.3.4: Dagryn Cloud Cache Service

**Goal:** Provide hosted remote cache for Dagryn Cloud users.

**Server components:**
```
internal/server/cache/
├── service.go         # Cache service implementation
├── handlers.go        # HTTP/gRPC handlers
├── storage.go         # Storage backend (S3/GCS)
├── quota.go           # Usage quotas and limits
└── gc.go              # Garbage collection
```

**Features:**
- Per-project cache isolation
- Usage quotas (storage size, bandwidth)
- Cache retention policies
- Cache analytics (hit rate, storage usage)
- Team/org cache sharing

**Database schema:**
```sql
CREATE TABLE cache_entries (
    id UUID PRIMARY KEY,
    project_id UUID REFERENCES projects(id),
    action_digest TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    hit_count INT DEFAULT 0,
    last_accessed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP,
    UNIQUE(project_id, action_digest)
);

CREATE TABLE cache_blobs (
    digest TEXT PRIMARY KEY,
    size_bytes BIGINT NOT NULL,
    storage_path TEXT NOT NULL,  -- S3 key
    ref_count INT DEFAULT 1,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE cache_usage (
    project_id UUID REFERENCES projects(id),
    date DATE,
    bytes_uploaded BIGINT DEFAULT 0,
    bytes_downloaded BIGINT DEFAULT 0,
    cache_hits INT DEFAULT 0,
    cache_misses INT DEFAULT 0,
    PRIMARY KEY (project_id, date)
);
```

**Tasks:**
- [ ] Implement cache service with Bazel Remote API
- [ ] Add S3/GCS storage backend
- [ ] Implement cache quotas and limits
- [ ] Add garbage collection for expired/unused entries
- [ ] Add cache analytics dashboard

---

# Part 2: GitHub Actions Integration

## 2.1 Goal

Allow users to run Dagryn workflows as GitHub Actions, enabling:
- Use Dagryn locally and in CI with the same config
- Leverage remote cache in CI
- View CI results in Dagryn dashboard

## 2.2 Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    GITHUB ACTIONS INTEGRATION                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  GitHub Repository                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  .github/workflows/dagryn.yml                                   │   │
│  │  ┌─────────────────────────────────────────────────────────┐    │   │
│  │  │  - uses: dagryn/action@v1                               │    │   │
│  │  │    with:                                                 │    │   │
│  │  │      targets: build test                                │    │   │
│  │  │      remote-cache: true                                 │    │   │
│  │  └─────────────────────────────────────────────────────────┘    │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                              │                                          │
│                              ▼                                          │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  Dagryn GitHub Action                                           │   │
│  │  1. Install dagryn CLI                                          │   │
│  │  2. Authenticate with Dagryn Cloud (optional)                   │   │
│  │  3. Run: dagryn run --sync --remote-cache <targets>            │   │
│  │  4. Report results back to GitHub                               │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                              │                                          │
│                              ▼                                          │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  Dagryn Server                                                  │   │
│  │  - Receives run sync                                            │   │
│  │  - Provides remote cache                                        │   │
│  │  - Stores run history                                           │   │
│  │  - Updates GitHub commit status                                 │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

## 2.3 Implementation Phases

### Phase 2.3.1: GitHub Action Definition

**Goal:** Create the official Dagryn GitHub Action.

**Repository:** `dagryn/action` (public)

**Files:**
```
dagryn-action/
├── action.yml           # Action metadata
├── src/
│   ├── main.ts         # Entry point
│   ├── installer.ts    # CLI installer
│   ├── runner.ts       # Run execution
│   ├── reporter.ts     # Status reporting
│   └── cache.ts        # GitHub Actions cache integration
├── dist/               # Compiled action
└── README.md
```

**action.yml:**
```yaml
name: 'Dagryn'
description: 'Run Dagryn workflows in GitHub Actions'
author: 'Dagryn'

branding:
  icon: 'zap'
  color: 'purple'

inputs:
  targets:
    description: 'Tasks or workflows to run'
    required: false
    default: ''
  config:
    description: 'Path to dagryn.toml'
    required: false
    default: 'dagryn.toml'
  version:
    description: 'Dagryn CLI version'
    required: false
    default: 'latest'
  remote-cache:
    description: 'Enable remote cache'
    required: false
    default: 'true'
  cloud-token:
    description: 'Dagryn Cloud API token'
    required: false
  project-id:
    description: 'Dagryn Cloud project ID'
    required: false
  working-directory:
    description: 'Working directory'
    required: false
    default: '.'

outputs:
  run-id:
    description: 'The Dagryn run ID'
  status:
    description: 'The run status (success, failed, cancelled)'
  duration:
    description: 'Total run duration in seconds'
  cache-hits:
    description: 'Number of cache hits'

runs:
  using: 'node20'
  main: 'dist/index.js'
```

**Tasks:**
- [ ] Create dagryn/action repository
- [ ] Implement CLI installer (with version pinning)
- [ ] Implement runner with proper error handling
- [ ] Implement GitHub Actions cache for local cache persistence
- [ ] Add support for matrix builds
- [ ] Add support for PR comments with results
- [ ] Add support for GitHub commit status updates

### Phase 2.3.2: GitHub App Integration

**Goal:** Create a GitHub App for deeper integration.

**Features:**
- Automatic workflow file generation
- PR checks with detailed task results
- Commit status updates
- Installation webhook handling

**GitHub App Permissions:**
```yaml
permissions:
  contents: read
  checks: write
  statuses: write
  pull-requests: write
  actions: read
```

**Webhook Events:**
```yaml
events:
  - push
  - pull_request
  - check_suite
  - installation
```

**Server handlers:**
```
internal/server/github/
├── app.go              # GitHub App client
├── webhooks.go         # Webhook handlers
├── checks.go           # Check runs API
├── installation.go     # Installation management
└── permissions.go      # Permission validation
```

**Tasks:**
- [ ] Create GitHub App configuration
- [ ] Implement installation webhook handler
- [ ] Implement push/PR webhook handlers
- [ ] Implement Check Runs API integration
- [ ] Add PR comment reporting
- [ ] Add workflow file generator

### Phase 2.3.3: Dagryn-Native CI (Server-Side Execution)

**Goal:** Run workflows on Dagryn infrastructure (alternative to GitHub Actions runners).

This expands on the existing PLAN_REMOTE_CACHE_AND_RUNS.md "Remote Run Execution" section.

**Flow:**
```
GitHub Push → Webhook → Dagryn Server → Job Queue → Runner → Results
```

**Runner architecture:**
```
internal/runner/
├── runner.go           # Runner orchestration
├── workspace.go        # Workspace management (clone, cleanup)
├── executor.go         # Task execution
├── reporter.go         # Status reporting
└── sandbox.go          # Execution isolation (containers, nsjail)
```

**Features:**
- Container-based isolation (Docker, Podman)
- Workspace per run (clone → execute → cleanup)
- Artifact collection and storage
- Log streaming to dashboard
- Concurrent run limits

**Tasks:**
- [ ] Implement runner service
- [ ] Add container-based isolation
- [ ] Implement workspace management
- [ ] Add artifact storage
- [ ] Implement log streaming
- [ ] Add runner scaling (multiple workers)

---

# Part 3: Dagryn Plugin Registry

## 3.1 Goal

Create a first-party plugin ecosystem where users can:
- Build plugins using a standard interface
- Publish plugins to the Dagryn registry
- Discover and install plugins from the registry
- Version and manage plugin dependencies

## 3.2 Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      DAGRYN PLUGIN ECOSYSTEM                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Plugin Author                        Plugin User                       │
│  ┌───────────────┐                    ┌───────────────┐                │
│  │ dagryn plugin │                    │ dagryn plugin │                │
│  │   publish     │                    │   install     │                │
│  └───────┬───────┘                    └───────┬───────┘                │
│          │                                    │                         │
│          ▼                                    ▼                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    DAGRYN PLUGIN REGISTRY                        │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │   │
│  │  │ Plugin API  │  │ Search &    │  │ Plugin Storage          │  │   │
│  │  │ (publish,   │  │ Discovery   │  │ (binaries, metadata)    │  │   │
│  │  │  versions)  │  │             │  │                         │  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────┘  │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    PLUGIN TYPES                                  │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │   │
│  │  │ CLI Tools   │  │ Task        │  │ Integrations            │  │   │
│  │  │ (binaries)  │  │ Templates   │  │ (providers, reporters)  │  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────┘  │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

## 3.3 Plugin Types

### Type 1: CLI Tool Plugins (Current + Enhanced)
Executable binaries that tasks can use. Already partially implemented.

### Type 2: Task Template Plugins (NEW)
Reusable task definitions that can be parameterized.

```toml
# Using a task template plugin
[tasks.lint]
uses = "dagryn/eslint-task@v1"
with:
  config = ".eslintrc.js"
  files = "src/**/*.ts"
```

### Type 3: Integration Plugins (NEW)
Extend Dagryn's capabilities (reporters, storage backends, etc.).

```toml
# Using integration plugins
[plugins]
slack-reporter = "dagryn/slack-reporter@v1"
s3-artifacts = "dagryn/s3-artifacts@v1"

[integrations.slack]
webhook_url_env = "SLACK_WEBHOOK_URL"
on = ["run.complete", "run.failed"]

[integrations.artifacts]
provider = "s3-artifacts"
bucket = "my-artifacts"
```

## 3.4 Implementation Phases

### Phase 3.4.1: Plugin Specification & SDK

**Goal:** Define the plugin specification and provide an SDK for building plugins.

**Plugin manifest (plugin.toml):**
```toml
[plugin]
name = "eslint"
version = "1.0.0"
description = "ESLint integration for Dagryn"
author = "Dagryn Team"
license = "MIT"
homepage = "https://github.com/dagryn/plugin-eslint"
repository = "https://github.com/dagryn/plugin-eslint"

# Plugin type: "tool", "task", "integration"
type = "tool"

# Keywords for search
keywords = ["linting", "javascript", "typescript"]

# Supported platforms (for tool plugins)
[platforms]
darwin-amd64 = "bin/eslint-darwin-amd64"
darwin-arm64 = "bin/eslint-darwin-arm64"
linux-amd64 = "bin/eslint-linux-amd64"
linux-arm64 = "bin/eslint-linux-arm64"
windows-amd64 = "bin/eslint-windows-amd64.exe"

# Dependencies on other plugins
[dependencies]
node = ">=18.0.0"

# Configuration schema (JSON Schema)
[config]
schema = "config.schema.json"
```

**Plugin SDK (Go):**
```go
// For integration plugins
package sdk

type Plugin interface {
    // Metadata returns plugin information
    Metadata() *Metadata

    // Initialize is called when the plugin is loaded
    Initialize(ctx context.Context, config map[string]any) error

    // Shutdown is called when the plugin is unloaded
    Shutdown(ctx context.Context) error
}

type ReporterPlugin interface {
    Plugin

    // OnRunStart is called when a run starts
    OnRunStart(ctx context.Context, run *Run) error

    // OnTaskComplete is called when a task completes
    OnTaskComplete(ctx context.Context, task *Task, result *Result) error

    // OnRunComplete is called when a run completes
    OnRunComplete(ctx context.Context, run *Run, summary *Summary) error
}

type ArtifactPlugin interface {
    Plugin

    // Upload uploads an artifact
    Upload(ctx context.Context, path string, metadata map[string]string) (*Artifact, error)

    // Download downloads an artifact
    Download(ctx context.Context, id string, destPath string) error

    // List lists artifacts for a run
    List(ctx context.Context, runID string) ([]*Artifact, error)
}
```

**Files to create:**
```
pkg/plugin-sdk/
├── plugin.go           # Core plugin interfaces
├── reporter.go         # Reporter plugin interface
├── artifact.go         # Artifact plugin interface
├── cache.go           # Cache plugin interface
├── config.go          # Configuration utilities
├── context.go         # Plugin context
└── testing/           # Testing utilities
    ├── mock.go
    └── harness.go
```

**Tasks:**
- [ ] Define plugin.toml specification
- [ ] Create plugin SDK package
- [ ] Implement plugin interfaces
- [ ] Add configuration validation (JSON Schema)
- [ ] Create plugin testing utilities
- [ ] Write plugin authoring documentation

### Phase 3.4.2: Plugin Registry Service

**Goal:** Build the registry service for storing and serving plugins.

**API Endpoints:**
```
# Plugin Management
POST   /api/v1/plugins                    # Publish new plugin
GET    /api/v1/plugins                    # List/search plugins
GET    /api/v1/plugins/:name              # Get plugin info
GET    /api/v1/plugins/:name/versions     # List versions
GET    /api/v1/plugins/:name/:version     # Get specific version
DELETE /api/v1/plugins/:name/:version     # Yank version (soft delete)

# Download
GET    /api/v1/plugins/:name/:version/download/:platform  # Download binary

# Publisher Management
POST   /api/v1/publishers                 # Create publisher (org/user)
GET    /api/v1/publishers/:name           # Get publisher info
PUT    /api/v1/publishers/:name/members   # Manage members
```

**Database schema:**
```sql
-- Publishers (users or orgs that can publish plugins)
CREATE TABLE publishers (
    id UUID PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,  -- e.g., "dagryn", "acme-corp"
    display_name TEXT,
    type TEXT NOT NULL,  -- "user" or "org"
    user_id UUID REFERENCES users(id),  -- if type = "user"
    team_id UUID REFERENCES teams(id),  -- if type = "org"
    verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Plugins
CREATE TABLE plugins (
    id UUID PRIMARY KEY,
    publisher_id UUID REFERENCES publishers(id),
    name TEXT NOT NULL,  -- e.g., "eslint"
    full_name TEXT UNIQUE NOT NULL,  -- e.g., "dagryn/eslint"
    type TEXT NOT NULL,  -- "tool", "task", "integration"
    description TEXT,
    homepage TEXT,
    repository TEXT,
    license TEXT,
    keywords TEXT[],
    downloads INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Plugin Versions
CREATE TABLE plugin_versions (
    id UUID PRIMARY KEY,
    plugin_id UUID REFERENCES plugins(id),
    version TEXT NOT NULL,
    manifest JSONB NOT NULL,  -- Full plugin.toml as JSON
    readme TEXT,
    changelog TEXT,
    checksum TEXT NOT NULL,  -- SHA256 of package
    size_bytes BIGINT,
    yanked BOOLEAN DEFAULT FALSE,
    published_by UUID REFERENCES users(id),
    published_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(plugin_id, version)
);

-- Plugin Binaries (for tool plugins)
CREATE TABLE plugin_binaries (
    id UUID PRIMARY KEY,
    version_id UUID REFERENCES plugin_versions(id),
    platform TEXT NOT NULL,  -- e.g., "darwin-arm64"
    storage_path TEXT NOT NULL,
    checksum TEXT NOT NULL,
    size_bytes BIGINT,
    UNIQUE(version_id, platform)
);

-- Plugin Downloads (analytics)
CREATE TABLE plugin_downloads (
    id UUID PRIMARY KEY,
    plugin_id UUID REFERENCES plugins(id),
    version_id UUID REFERENCES plugin_versions(id),
    platform TEXT,
    user_id UUID REFERENCES users(id),
    ip_address INET,
    downloaded_at TIMESTAMP DEFAULT NOW()
);
```

**Server implementation:**
```
internal/server/registry/
├── service.go          # Registry service
├── handlers.go         # HTTP handlers
├── publisher.go        # Publisher management
├── plugin.go           # Plugin CRUD
├── version.go          # Version management
├── storage.go          # Binary storage (S3)
├── search.go           # Search/discovery
└── validation.go       # Manifest validation
```

**Tasks:**
- [ ] Design and create database schema
- [ ] Implement publisher management
- [ ] Implement plugin CRUD operations
- [ ] Implement version management
- [ ] Add binary storage with S3
- [ ] Implement search with full-text search
- [ ] Add download tracking and analytics
- [ ] Implement rate limiting

### Phase 3.4.3: Plugin CLI Commands

**Goal:** Add CLI commands for plugin management.

**Commands:**
```bash
# Discovery
dagryn plugin search <query>        # Search for plugins
dagryn plugin info <name>           # Show plugin details
dagryn plugin list                  # List installed plugins

# Installation
dagryn plugin install <name>[@version]   # Install a plugin
dagryn plugin update [name]              # Update plugins
dagryn plugin uninstall <name>           # Remove a plugin

# Publishing
dagryn plugin init                  # Initialize a new plugin project
dagryn plugin build                 # Build plugin for all platforms
dagryn plugin pack                  # Package plugin for publishing
dagryn plugin publish               # Publish to registry

# Authentication
dagryn plugin login                 # Authenticate with registry
dagryn plugin logout                # Remove authentication
dagryn plugin whoami                # Show current user

# Publisher Management
dagryn plugin publisher create <name>    # Create a publisher
dagryn plugin publisher add-member       # Add team member
```

**Files to create:**
```
internal/cli/
├── plugin_search.go
├── plugin_info.go
├── plugin_install.go
├── plugin_publish.go
├── plugin_init.go
├── plugin_build.go
└── plugin_auth.go
```

**Tasks:**
- [ ] Implement search command
- [ ] Implement install/uninstall commands
- [ ] Implement publish workflow
- [ ] Implement plugin init scaffolding
- [ ] Add multi-platform build support
- [ ] Implement authentication flow

### Phase 3.4.4: Plugin Resolution & Loading

**Goal:** Update the plugin manager to support registry plugins.

**Updated plugin specification format:**
```toml
[plugins]
# Registry plugins (new format)
eslint = "dagryn/eslint@^1.0.0"
prettier = "dagryn/prettier@2.0.0"

# Legacy formats (still supported)
golangci-lint = "github:golangci/golangci-lint@v1.55.0"
goimports = "go:golang.org/x/tools/cmd/goimports@latest"

# Local plugins (for development)
my-plugin = "local:./plugins/my-plugin"
```

**Resolution priority:**
1. Check if it's a registry plugin (format: `publisher/name@version`)
2. Check if it's a legacy format (`source:name@version`)
3. Check if it's a local plugin (`local:path`)

**Files to modify:**
```
internal/plugin/
├── plugin.go           # Add registry source type
├── resolver.go         # Add registry resolver
├── registry.go         # Registry client (NEW)
├── manager.go          # Update to support registry
└── loader.go           # Plugin loading (NEW)
```

**Tasks:**
- [ ] Add registry source type
- [ ] Implement registry resolver
- [ ] Implement registry client
- [ ] Update manager for registry plugins
- [ ] Add plugin loading for integration plugins
- [ ] Implement version constraint resolution

### Phase 3.4.5: Plugin Web Interface

**Goal:** Build a web interface for browsing and managing plugins.

**Pages:**
```
/plugins                    # Plugin directory
/plugins/:publisher/:name   # Plugin detail page
/plugins/publish            # Publish new plugin
/settings/plugins           # Manage installed plugins
```

**Features:**
- Plugin search and filtering
- Plugin detail with README, versions, stats
- One-click install (generates config snippet)
- Publisher profiles
- Trending and popular plugins

**Tasks:**
- [ ] Design plugin directory UI
- [ ] Implement plugin detail page
- [ ] Add search and filtering
- [ ] Implement publisher profiles
- [ ] Add install code snippets
- [ ] Add plugin analytics dashboard

---

# Part 4: Implementation Order

## Recommended Order

```
Phase 1: Foundation (Weeks 1-4)
├── 1.3.1: Remote Cache Protocol
├── 1.3.2: Cache Backend Abstraction
├── 3.4.1: Plugin Specification & SDK
└── 3.4.2: Plugin Registry Service (DB schema)

Phase 2: Core Features (Weeks 5-8)
├── 1.3.3: Cache Configuration & CLI
├── 2.3.1: GitHub Action Definition
├── 3.4.2: Plugin Registry Service (full)
└── 3.4.3: Plugin CLI Commands

Phase 3: Integration (Weeks 9-12)
├── 1.3.4: Dagryn Cloud Cache Service
├── 2.3.2: GitHub App Integration
├── 3.4.4: Plugin Resolution & Loading
└── 3.4.5: Plugin Web Interface

Phase 4: Advanced (Weeks 13-16)
├── 2.3.3: Dagryn-Native CI
├── Integration plugin system
└── Task template plugins
```

## Dependencies

```
Remote Cache Protocol ──┬──→ Cache Backend ──→ Cloud Cache Service
                       │
                       └──→ GitHub Action (uses remote cache)

Plugin SDK ──→ Plugin Registry ──→ Plugin CLI ──→ Plugin Web UI
                     │
                     └──→ Plugin Resolution (manager update)

GitHub Action ──→ GitHub App ──→ Native CI
```

---

# Part 5: API Reference

## Remote Cache API

```protobuf
// Using Bazel Remote Execution API
service ActionCache {
  rpc GetActionResult(GetActionResultRequest) returns (ActionResult);
  rpc UpdateActionResult(UpdateActionResultRequest) returns (ActionResult);
}

service ContentAddressableStorage {
  rpc FindMissingBlobs(FindMissingBlobsRequest) returns (FindMissingBlobsResponse);
  rpc BatchUpdateBlobs(BatchUpdateBlobsRequest) returns (BatchUpdateBlobsResponse);
  rpc BatchReadBlobs(BatchReadBlobsRequest) returns (BatchReadBlobsResponse);
}
```

## Plugin Registry API

```yaml
openapi: 3.0.0
paths:
  /api/v1/plugins:
    get:
      summary: List/search plugins
      parameters:
        - name: q
          in: query
          description: Search query
        - name: type
          in: query
          enum: [tool, task, integration]
        - name: sort
          in: query
          enum: [downloads, updated, name]
    post:
      summary: Publish a new plugin
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                manifest:
                  type: string
                  format: binary
                binaries:
                  type: array
                  items:
                    type: string
                    format: binary

  /api/v1/plugins/{publisher}/{name}:
    get:
      summary: Get plugin info

  /api/v1/plugins/{publisher}/{name}/{version}/download/{platform}:
    get:
      summary: Download plugin binary
```

---

# Part 6: Security Considerations

## Remote Cache
- API key or mTLS authentication
- Per-project cache isolation
- Content verification (digest matching)
- Encryption at rest and in transit

## Plugin Registry
- Package signing (optional)
- Malware scanning
- Verified publishers
- Dependency vulnerability scanning
- Rate limiting
- Audit logging

## GitHub Integration
- Webhook signature verification
- Minimal permission scope
- Token encryption
- Installation validation

---

# Part 7: References

- [Bazel Remote APIs](https://github.com/bazelbuild/remote-apis)
- [GitHub Actions Toolkit](https://github.com/actions/toolkit)
- [npm Registry API](https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md)
- [Cargo Registry](https://doc.rust-lang.org/cargo/reference/registries.html)
- Current plugin system: `internal/plugin/`
- Existing plans: `docs/PLAN_REMOTE_CACHE_AND_RUNS.md`
