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
│  ┌────────────────┐ ┌─────────────────────────────────┐ ┌───────────────┐  │
│  │ Job Queue      │ │      UNIFIED STORAGE LAYER      │ │ Git Providers │  │
│  │ (Redis/NATS)   │ │  ┌─────┐ ┌────┐ ┌─────┐ ┌────┐ │ │ (GH/GL/BB)    │  │
│  └────────────────┘ │  │ S3  │ │ R2 │ │ GCS │ │MinIO│ │ └───────────────┘  │
│                     │  └─────┘ └────┘ └─────┘ └────┘ │                     │
│                     └─────────────────────────────────┘                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

# Part 0: Unified Storage Layer (Foundation)

## 0.1 Goal

Provide a **single, pluggable storage abstraction** that all Dagryn services use. Users can easily swap between storage providers (S3, R2, GCS, MinIO, local filesystem) with a simple configuration change.

## 0.2 Design Principles

1. **Single Interface** - One interface for all storage operations
2. **Zero Code Changes** - Swap providers via config only
3. **Provider Parity** - All providers support the same features
4. **Sensible Defaults** - Works out of the box with local storage
5. **Cloud Native** - First-class support for S3-compatible APIs

## 0.3 Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      UNIFIED STORAGE LAYER                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Consumers (use storage.Bucket interface)                               │
│  ┌────────────────┐ ┌────────────────┐ ┌────────────────────────────┐  │
│  │ Remote Cache   │ │ Plugin Registry│ │ Artifacts / Logs           │  │
│  │ (blobs, AC)    │ │ (binaries)     │ │ (run outputs)              │  │
│  └───────┬────────┘ └───────┬────────┘ └────────────┬───────────────┘  │
│          │                  │                       │                   │
│          └──────────────────┼───────────────────────┘                   │
│                             ▼                                           │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    storage.Bucket Interface                      │   │
│  │  Put() | Get() | Delete() | List() | Exists() | SignedURL()     │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                             │                                           │
│          ┌──────────────────┼──────────────────────┐                   │
│          ▼                  ▼                      ▼                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │  S3 Provider │  │  R2 Provider │  │ GCS Provider │  │   Local    │ │
│  │  (AWS S3)    │  │ (Cloudflare) │  │  (Google)    │  │ Filesystem │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └────────────┘ │
│          │                  │                │                │        │
│          │         ┌────────┴────────┐       │                │        │
│          │         │  S3-Compatible  │       │                │        │
│          │         │   (MinIO, etc)  │       │                │        │
│          │         └─────────────────┘       │                │        │
│          ▼                  ▼                ▼                ▼        │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                     Actual Storage                               │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

## 0.4 Implementation

### Phase 0.4.1: Storage Interface

**Files to create:**

```
pkg/storage/
├── storage.go          # Core interfaces
├── config.go           # Configuration types
├── errors.go           # Storage-specific errors
├── providers/
│   ├── s3/
│   │   ├── s3.go       # AWS S3 implementation
│   │   └── config.go
│   ├── r2/
│   │   ├── r2.go       # Cloudflare R2 implementation
│   │   └── config.go
│   ├── gcs/
│   │   ├── gcs.go      # Google Cloud Storage implementation
│   │   └── config.go
│   ├── minio/
│   │   ├── minio.go    # MinIO implementation
│   │   └── config.go
│   ├── azure/
│   │   ├── azure.go    # Azure Blob Storage implementation
│   │   └── config.go
│   └── filesystem/
│       ├── fs.go       # Local filesystem implementation
│       └── config.go
└── factory.go          # Provider factory
```

**Core interface:**

```go
package storage

import (
    "context"
    "io"
    "time"
)

// Bucket represents a storage bucket/container
type Bucket interface {
    // Put uploads data to the bucket
    Put(ctx context.Context, key string, r io.Reader, opts *PutOptions) error

    // PutBytes is a convenience method for uploading byte slices
    PutBytes(ctx context.Context, key string, data []byte, opts *PutOptions) error

    // Get retrieves data from the bucket
    Get(ctx context.Context, key string) (io.ReadCloser, error)

    // GetBytes is a convenience method for downloading to byte slice
    GetBytes(ctx context.Context, key string) ([]byte, error)

    // Delete removes an object from the bucket
    Delete(ctx context.Context, key string) error

    // DeleteMany removes multiple objects (batch delete)
    DeleteMany(ctx context.Context, keys []string) error

    // Exists checks if an object exists
    Exists(ctx context.Context, key string) (bool, error)

    // List returns objects matching the prefix
    List(ctx context.Context, prefix string, opts *ListOptions) (*ListResult, error)

    // Head returns object metadata without downloading
    Head(ctx context.Context, key string) (*ObjectInfo, error)

    // SignedURL generates a pre-signed URL for direct access
    SignedURL(ctx context.Context, key string, opts *SignedURLOptions) (string, error)

    // Copy copies an object within or between buckets
    Copy(ctx context.Context, srcKey, dstKey string) error
}

// PutOptions configures upload behavior
type PutOptions struct {
    ContentType     string
    ContentEncoding string
    CacheControl    string
    Metadata        map[string]string
    // Server-side encryption
    Encryption      *EncryptionConfig
}

// ListOptions configures list behavior
type ListOptions struct {
    MaxKeys   int
    Delimiter string
    StartAfter string
}

// ListResult contains list operation results
type ListResult struct {
    Objects       []ObjectInfo
    Prefixes      []string  // "directories" when using delimiter
    IsTruncated   bool
    NextMarker    string
}

// ObjectInfo contains object metadata
type ObjectInfo struct {
    Key          string
    Size         int64
    LastModified time.Time
    ETag         string
    ContentType  string
    Metadata     map[string]string
}

// SignedURLOptions configures signed URL generation
type SignedURLOptions struct {
    Method   string        // GET, PUT
    Expires  time.Duration
    // For PUT: content type requirement
    ContentType string
}

// EncryptionConfig for server-side encryption
type EncryptionConfig struct {
    Type      string // "AES256", "aws:kms", etc.
    KMSKeyID  string // For KMS encryption
}
```

**Provider factory:**

```go
package storage

import (
    "fmt"
)

// ProviderType identifies a storage provider
type ProviderType string

const (
    ProviderS3         ProviderType = "s3"
    ProviderR2         ProviderType = "r2"
    ProviderGCS        ProviderType = "gcs"
    ProviderMinio      ProviderType = "minio"
    ProviderAzure      ProviderType = "azure"
    ProviderFilesystem ProviderType = "filesystem"
)

// Config is the unified storage configuration.
// All providers share common fields; provider-specific mapping:
//   - Azure: AccessKeyID = account name, SecretAccessKey = account key,
//            Endpoint = account URL, Bucket = container name
//   - R2:    Endpoint = https://<account_id>.r2.cloudflarestorage.com
//   - GCS:   CredentialsFile = path to service account JSON
//   - MinIO: UsePathStyle = true (set automatically by factory)
type Config struct {
    Provider        ProviderType
    BasePath        string // filesystem: root directory
    Bucket          string // s3/gcs/azure: bucket/container name
    Region          string // s3/gcs: region
    Endpoint        string // s3/azure: custom endpoint (MinIO, R2, Azure account URL)
    AccessKeyID     string
    SecretAccessKey string
    UsePathStyle    bool   // s3: path-style addressing (auto-set for MinIO)
    Prefix          string // key prefix applied to all operations
    CredentialsFile string // gcs: path to service account JSON
}

// NewBucket creates a storage bucket based on configuration
func NewBucket(cfg Config) (Bucket, error) {
    switch cfg.Provider {
    case ProviderS3:
        return newS3Bucket(cfg)
    case ProviderR2:
        return newR2Bucket(cfg)
    case ProviderGCS:
        return newGCSBucket(cfg)
    case ProviderMinio:
        return newMinioBucket(cfg)
    case ProviderAzure:
        return newAzureBucket(cfg)
    case ProviderFilesystem, "":
        return newFilesystemBucket(cfg)
    default:
        return nil, fmt.Errorf("unknown storage provider: %s", cfg.Provider)
    }
}
```

### Phase 0.4.2: Provider Implementations

**S3 Provider (also base for R2, MinIO):**

```go
package s3

import (
    "context"
    "io"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/mujhtech/dagryn/pkg/storage"
)

type Bucket struct {
    client *s3.Client
    bucket string
    prefix string
}

func New(cfg storage.Config) (*Bucket, error) {
    // Build AWS config
    opts := []func(*config.LoadOptions) error{
        config.WithRegion(cfg.Region),
    }

    // Custom endpoint (for R2, MinIO, etc.)
    if cfg.Endpoint != "" {
        opts = append(opts, config.WithEndpointResolverWithOptions(
            // ... custom endpoint resolver
        ))
    }

    // Static credentials
    if cfg.AccessKeyID != "" {
        opts = append(opts, config.WithCredentialsProvider(
            credentials.NewStaticCredentialsProvider(
                cfg.AccessKeyID,
                cfg.SecretAccessKey,
                "",
            ),
        ))
    }

    awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
    if err != nil {
        return nil, err
    }

    client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
        if cfg.UsePathStyle {
            o.UsePathStyle = true
        }
    })

    return &Bucket{
        client: client,
        bucket: cfg.Bucket,
        prefix: cfg.Prefix,
    }, nil
}

func (b *Bucket) Put(ctx context.Context, key string, r io.Reader, opts *storage.PutOptions) error {
    input := &s3.PutObjectInput{
        Bucket: &b.bucket,
        Key:    aws.String(b.prefix + key),
        Body:   r,
    }
    if opts != nil {
        if opts.ContentType != "" {
            input.ContentType = &opts.ContentType
        }
        // ... other options
    }
    _, err := b.client.PutObject(ctx, input)
    return err
}

// ... implement other interface methods
```

**Cloudflare R2 Provider:**

```go
package r2

import (
    "github.com/mujhtech/dagryn/pkg/storage"
    "github.com/mujhtech/dagryn/pkg/storage/providers/s3"
)

// R2 is S3-compatible, so we wrap the S3 provider
func New(cfg storage.Config) (*s3.Bucket, error) {
    // R2 endpoint format: https://<account_id>.r2.cloudflarestorage.com
    if cfg.Endpoint == "" && cfg.AccountID != "" {
        cfg.Endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
    }
    cfg.UsePathStyle = true
    cfg.Region = "auto"

    return s3.New(cfg)
}
```

**Local Filesystem Provider:**

```go
package filesystem

import (
    "context"
    "io"
    "os"
    "path/filepath"

    "github.com/mujhtech/dagryn/pkg/storage"
)

type Bucket struct {
    basePath string
}

func New(cfg storage.Config) (*Bucket, error) {
    basePath := cfg.BasePath
    if basePath == "" {
        basePath = ".dagryn/storage"
    }

    // Ensure directory exists
    if err := os.MkdirAll(basePath, 0755); err != nil {
        return nil, err
    }

    return &Bucket{basePath: basePath}, nil
}

func (b *Bucket) Put(ctx context.Context, key string, r io.Reader, opts *storage.PutOptions) error {
    path := filepath.Join(b.basePath, key)

    // Ensure parent directory exists
    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        return err
    }

    f, err := os.Create(path)
    if err != nil {
        return err
    }
    defer f.Close()

    _, err = io.Copy(f, r)
    return err
}

// ... implement other interface methods
```

### Phase 0.4.3: Configuration

**Server configuration (dagryn.server.toml):**

There are two storage configuration scopes:

1. **Generic storage** (`pkg/storage.ConfigFromEnv`) — reads `DAGRYN_STORAGE_*` env vars, used by CLI-side components
2. **Cache storage** (`[cache_storage]` in server config) — reads `DAGRYN_CACHE_STORAGE_*` env vars, used by the cloud cache service

```toml
# Cache storage configuration — used by cloud cache service
# Provider: "s3", "r2", "gcs", "minio", "azure", "filesystem"
[cache_storage]
provider = "s3"
bucket = "dagryn-cache"
region = "us-east-1"
prefix = "v1/"
access_key_id = ""      # or use DAGRYN_CACHE_STORAGE_ACCESS_KEY_ID env
secret_access_key = ""  # or use DAGRYN_CACHE_STORAGE_SECRET_ACCESS_KEY env

# For R2: endpoint = "https://<account_id>.r2.cloudflarestorage.com"
# For GCS: credentials_file = "/path/to/service-account.json"
# For Azure: endpoint = "https://<account>.blob.core.windows.net"
#            access_key_id = account name, secret_access_key = account key
# For MinIO: endpoint = "http://minio.local:9000", use_path_style = true
# For filesystem: base_path = "/var/dagryn/cache-storage"
```

**Environment variable support:**

Generic storage (CLI / `pkg/storage`):

```bash
DAGRYN_STORAGE_PROVIDER=filesystem
DAGRYN_STORAGE_BASE_PATH=/var/dagryn/storage
DAGRYN_STORAGE_BUCKET=my-bucket
DAGRYN_STORAGE_REGION=us-west-2
DAGRYN_STORAGE_ENDPOINT=http://minio.local:9000
DAGRYN_STORAGE_ACCESS_KEY_ID=xxx
DAGRYN_STORAGE_SECRET_ACCESS_KEY=xxx
DAGRYN_STORAGE_PREFIX=v1/
DAGRYN_STORAGE_CREDENTIALS_FILE=/path/to/sa.json
```

Cache storage (server / `[cache_storage]`):

```bash
DAGRYN_CACHE_STORAGE_PROVIDER=s3
DAGRYN_CACHE_STORAGE_BUCKET=dagryn-cache
DAGRYN_CACHE_STORAGE_REGION=us-east-1
DAGRYN_CACHE_STORAGE_ENDPOINT=
DAGRYN_CACHE_STORAGE_ACCESS_KEY_ID=xxx
DAGRYN_CACHE_STORAGE_SECRET_ACCESS_KEY=xxx
DAGRYN_CACHE_STORAGE_BASE_PATH=
DAGRYN_CACHE_STORAGE_PREFIX=v1/
DAGRYN_CACHE_STORAGE_CREDENTIALS_FILE=
DAGRYN_CACHE_STORAGE_USE_PATH_STYLE=false
```

### Phase 0.4.4: Storage Manager

**Unified storage manager for the server:**

```go
package storage

// Manager provides named buckets for different purposes
type Manager struct {
    defaultBucket Bucket
    buckets       map[string]Bucket
}

func NewManager(cfg Config, overrides map[string]Config) (*Manager, error) {
    defaultBucket, err := NewBucket(cfg)
    if err != nil {
        return nil, fmt.Errorf("failed to create default bucket: %w", err)
    }

    m := &Manager{
        defaultBucket: defaultBucket,
        buckets:       make(map[string]Bucket),
    }

    // Create override buckets (cache, plugins, artifacts)
    for name, overrideCfg := range overrides {
        // Merge with default config
        merged := mergeConfig(cfg, overrideCfg)
        bucket, err := NewBucket(merged)
        if err != nil {
            return nil, fmt.Errorf("failed to create %s bucket: %w", name, err)
        }
        m.buckets[name] = bucket
    }

    return m, nil
}

// Bucket returns a named bucket or the default
func (m *Manager) Bucket(name string) Bucket {
    if b, ok := m.buckets[name]; ok {
        return b
    }
    return m.defaultBucket
}

// Cache returns the cache bucket
func (m *Manager) Cache() Bucket {
    return m.Bucket("cache")
}

// Plugins returns the plugins bucket
func (m *Manager) Plugins() Bucket {
    return m.Bucket("plugins")
}

// Artifacts returns the artifacts bucket
func (m *Manager) Artifacts() Bucket {
    return m.Bucket("artifacts")
}
```

**Tasks:**

- [x] Define storage.Bucket interface — `pkg/storage/storage.go` (simplified: `Put`, `Get`, `Delete`, `Exists`, `List` — no `Head`/`Copy`/`PutBytes`/`GetBytes`/`DeleteMany`)
- [x] Implement S3 provider — `pkg/storage/s3.go` (covers S3; R2/MinIO via factory presets)
- [x] Implement R2 provider (factory preset) — `pkg/storage/factory.go` (`ProviderR2` → `UsePathStyle=false`, delegates to `NewS3Bucket`)
- [x] Implement GCS provider — `pkg/storage/gcs.go` (`GCSBucket` using `cloud.google.com/go/storage`, implements `Bucket` + `SignedURLer`)
- [x] Implement MinIO provider (factory preset) — `pkg/storage/factory.go` (`ProviderMinIO` → `UsePathStyle=true`, delegates to `NewS3Bucket`)
- [x] Implement Azure Blob provider — `pkg/storage/azure.go` (`AzureBucket` using `azblob` SDK, shared key + DefaultAzureCredential, implements `Bucket` + `SignedURLer`)
- [x] Implement filesystem provider — `pkg/storage/filesystem.go`
- [x] Create provider factory — `pkg/storage/factory.go` (`NewBucket(cfg Config)` with S3, GCS, Azure, R2, MinIO, filesystem cases)
- [x] Implement storage manager — `pkg/storage/manager.go` (thread-safe `Manager` with `Register`, `SetPrimary`, `Get`, `Primary`, `Names`)
- [x] Add configuration parsing — `pkg/storage/config.go` (`Config` with `CredentialsFile` for GCS)
- [x] Add environment variable support — `pkg/storage/config.go` (`ConfigFromEnv()` reads `DAGRYN_STORAGE_*` env vars)
- [x] Write provider tests — `pkg/storage/filesystem_test.go` (6 tests), `pkg/storage/s3_test.go` (7 tests), `pkg/storage/gcs_test.go` (4 tests), `pkg/storage/azure_test.go` (5 tests), `pkg/storage/manager_test.go` (6 tests)
- [x] Add signed URL support — `pkg/storage/storage.go` (`SignedURLer` optional interface), implemented on `S3Bucket`, `GCSBucket`, `AzureBucket`

> **Implementation Note:** The storage layer was implemented as a flat `pkg/storage/` package (no `providers/` subdirectories). R2 and MinIO are factory presets that delegate to `NewS3Bucket` with appropriate options. GCS and Azure are full provider implementations with their own SDKs. The `SignedURLer` interface is optional — providers that support pre-signed URLs implement it separately from `Bucket`. The `Manager` supports named multi-bucket setups for cache, artifacts, etc. `ConfigFromEnv()` reads `DAGRYN_STORAGE_PROVIDER`, `DAGRYN_STORAGE_BUCKET`, `DAGRYN_STORAGE_REGION`, etc.

## 0.5 Usage Examples

**Remote Cache using storage:**

```go
// internal/cache/remote/storage.go
type StorageBackend struct {
    bucket storage.Bucket
}

func (s *StorageBackend) SaveBlob(ctx context.Context, digest *Digest, data []byte) error {
    key := fmt.Sprintf("cas/%s/%s", digest.Hash[:2], digest.Hash)
    return s.bucket.PutBytes(ctx, key, data, &storage.PutOptions{
        ContentType: "application/octet-stream",
    })
}

func (s *StorageBackend) GetBlob(ctx context.Context, digest *Digest) ([]byte, error) {
    key := fmt.Sprintf("cas/%s/%s", digest.Hash[:2], digest.Hash)
    return s.bucket.GetBytes(ctx, key)
}
```

**Plugin Registry using storage:**

```go
// internal/server/registry/storage.go
type PluginStorage struct {
    bucket storage.Bucket
}

func (s *PluginStorage) UploadBinary(ctx context.Context, plugin, version, platform string, data []byte) error {
    key := fmt.Sprintf("plugins/%s/%s/%s.tar.gz", plugin, version, platform)
    return s.bucket.PutBytes(ctx, key, data, &storage.PutOptions{
        ContentType: "application/gzip",
    })
}

func (s *PluginStorage) GetDownloadURL(ctx context.Context, plugin, version, platform string) (string, error) {
    key := fmt.Sprintf("plugins/%s/%s/%s.tar.gz", plugin, version, platform)
    return s.bucket.SignedURL(ctx, key, &storage.SignedURLOptions{
        Method:  "GET",
        Expires: 1 * time.Hour,
    })
}
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

- [x] Implement Digest calculation — `internal/cache/remote/digest.go` (SHA256-based, CAS key: `cas/{hash[:2]}/{hash}`)
- [x] Implement Action encoding — `internal/cache/remote/action.go` (`ActionKey(taskName, cacheKey)` → `ac/{taskName}/{cacheKey}`)
- [x] Implement CAS storage — `internal/cache/remote/backend.go` (direct object storage via `pkg/storage.Bucket`, not gRPC/Bazel Remote APIs)
- [x] Implement ActionCache — `internal/cache/remote/manifest.go` (JSON manifest at action key mapping file paths to digests)
- [x] Add gRPC connection management with retry and timeout — `internal/cache/grpc/conn.go` (REAPI v2 via gRPC with retry, keepalive, TLS, bearer auth)
- [x] Add authentication support (API key, mTLS, OAuth) — `internal/cache/grpc/conn.go` (bearer token auth + custom CA cert support)

> **Implementation Note:** Instead of Bazel Remote Execution gRPC APIs, the remote cache uses direct object storage via `pkg/storage.Bucket`. This is simpler, requires no server component, and works with any S3-compatible storage. CAS deduplication is still achieved — blobs are stored at content-addressable keys, and manifests map file paths to digests.

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

- [x] Define Backend interface — `internal/cache/backend.go` (`Check`, `Restore`, `Save`, `Clear`, `ClearAll` with `context.Context`)
- [x] Refactor local cache to implement Backend — `internal/cache/local.go` (`LocalBackend` wrapping `Store`)
- [x] Implement remote backend using storage — `internal/cache/remote/backend.go` (`StorageBackend` implementing `cache.Backend`)
- [x] Implement HybridBackend with configurable strategies — `internal/cache/hybrid.go` (local-first, remote-first, write-through + FallbackOnError + local cache warming)
- [x] Update scheduler to use Backend abstraction — `internal/scheduler/scheduler.go` (`Options.CacheBackend`, `NewWithBackend()`)

> **Implementation Note:** The `Backend` interface uses `(taskName, key string)` params instead of a single `key string`. The `Cache` struct wraps `Backend` and adds task-level logic (hash computation, enabled check). `cache.New()` remains backward-compatible; `cache.NewWithBackend()` accepts custom backends. All methods received `context.Context` as first param. HybridBackend supports 3 strategies with `warmLocal()` for local cache warming after remote restore. 22 unit tests in `hybrid_test.go`.

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

- [x] Add cache configuration to config schema — `internal/config/schema.go` (`CacheConfig`, `RemoteCacheConfig` with `*bool` defaults-to-true pattern)
- [x] Add CLI flags for cache control — `internal/cli/root.go` (`--no-remote-cache` global flag)
- [x] Add `dagryn cache` subcommands — `internal/cli/cache.go` (`status`, `clear`, `push`, `pull` — all fully implemented)
- [x] Add cache statistics and reporting — `cache status` shows entry count + remote connectivity; `cache push/pull` show progress

> **Implementation Note:** Config uses `*bool` fields with `IsEnabled()`/`IsFallbackOnError()` methods to default to `true` when not set. `cache push` walks local entries via `Store.ListEntries()`, temporarily restores outputs to project root for upload, then cleans up. `cache pull` lists remote `ac/` prefix keys, restores via `StorageBackend.Restore`, and creates local metadata markers. Config validation added in `internal/config/validator.go` for provider, required fields, and strategy values.

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

- [x] Implement cache service — `internal/service/cache.go` (`CacheService` with Check/Upload/Download/Delete/GetStats/RunGC/GetAnalytics; CAS dedup via SHA256, TeeReader hashing, quota enforcement)
- [x] Add multi-cloud storage backend — `pkg/storage/` (S3, GCS, Azure, R2, MinIO, filesystem providers; server wired via `DAGRYN_CACHE_STORAGE_*` env vars)
- [x] Implement cache quotas and limits — `internal/db/repo/cache.go` (`cache_quotas` table with max_size_bytes/max_entries, `EnsureQuota`, `UpdateQuotaUsage`; Upload checks quota before storing)
- [x] Add garbage collection for expired/unused entries — `internal/service/cache.go` (`RunGC`: expired entry removal → LRU eviction when over quota → orphaned blob cleanup); `internal/job/handlers/cache_gc.go` (hourly background job via asynq iterating all projects)
- [x] Add cache analytics dashboard — backend: `internal/db/migrations/023_cache_usage.sql` (daily usage tracking table), `internal/db/repo/cache.go` (`IncrementUsage`, `GetUsageAnalytics`), `internal/service/cache.go` (`GetAnalytics`, usage recording in Check/Upload/Download), `internal/server/handlers/cache.go` (`GetCacheAnalytics` handler at `GET /cache/analytics?days=N`); frontend: `web/app/routes/projects/$projectId/cache.tsx` (stats cards, hit rate chart, bandwidth chart, top tasks table, daily usage table with day range selector)
- [x] Add DB migration for cache entries/blobs/quotas — `internal/db/migrations/022_cache_entries.sql` (3 tables, 4 indexes)
- [x] Add DB models — `internal/db/models/cache.go` (`CacheEntry`, `CacheBlob`, `CacheQuota`, `CacheUsage`)
- [x] Add cache repository — `internal/db/repo/cache.go` (`CacheRepo` with 17 methods: FindEntry, UpsertEntry, DeleteEntry, ListEntries, IncrementHitCount, UpsertBlob, DecrementBlobRef, ListOrphanedBlobs, DeleteBlob, GetQuota, EnsureQuota, UpdateQuotaUsage, GetStats, ListExpired, ListLRU, IncrementUsage, GetUsageAnalytics)
- [x] Add HTTP handlers + routes — `internal/server/handlers/cache.go` (7 handlers: CheckCache, UploadCache, DownloadCache, DeleteCache, GetCacheStats, TriggerCacheGC, GetCacheAnalytics), `internal/server/routes.go` (mounted under `/api/v1/projects/{projectId}/cache/`)
- [x] Wire into server — `internal/server/server.go` (storage bucket init from config, CacheRepo + CacheService creation, injected into Handler), `internal/server/config.go` (`CacheStorage StorageConfig` with `DAGRYN_CACHE_STORAGE_*` env vars)
- [x] Add frontend query hooks — `web/app/hooks/queries/use-cache-stats.ts`, `use-cache-analytics.ts`, API types in `web/app/lib/api.ts`

> **Implementation Note:** The cloud cache service uses a 3-layer architecture: HTTP handlers → CacheService → CacheRepo + storage.Bucket. Content dedup via CAS (blobs stored at `blobs/{hash[:2]}/{hash}`). Upload uses TeeReader to hash while writing to temp key, then moves to CAS location. Download increments hit count fire-and-forget via goroutine. GC runs as hourly asynq job — iterates all projects, removes expired entries, evicts LRU when over quota, deletes orphaned blobs. Usage analytics tracked daily in `cache_usage` table (hits, misses, bytes up/down) with upsert-on-conflict for atomic counter increments. The frontend dashboard at `/projects/$projectId/cache` shows 4 stats cards (hit rate, storage, entries, bandwidth), stacked bar chart for hits/misses, area chart for bandwidth, top tasks by size, and a daily usage table.

### Phase 1.3.5: Fix Recursive Glob & Workdir-Aware Caching

**Status:** IN PROGRESS

**Problem:**
Go's `filepath.Glob` treats `**` as a single-level wildcard (equivalent to `*`), not as recursive descent. This breaks caching for patterns like `node_modules/**`, `dist/**`, `vendor/**` which intend to match the full directory tree. Additionally, tasks with `workdir` had their input/output patterns resolved relative to project root instead of the workdir.

**Impact:**

1. `node_modules/**` only matches top-level entries (e.g., `node_modules/typescript/`) but NOT nested files (e.g., `node_modules/typescript/bin/tsc`)
2. `Store.Save` (local cache) calls `copyFile` on directory entries, which silently fails — nothing is actually cached
3. `createArchive` (cloud cache tar.go) partially works because `addDirToTar` walks matched directories recursively, but only captures directories matched by the single-level `**` glob
4. `HashFiles` (input hashing) misses nested files, producing incomplete hashes
5. Tasks with `workdir = "web"` and `outputs = ["node_modules/**"]` would look for `<root>/node_modules/**` instead of `<root>/web/node_modules/**`

**Completed fixes (workdir):**

- [x] `internal/cache/hasher.go` — `HashTask` resolves input patterns relative to `filepath.Join(projectRoot, t.Workdir)` when workdir is set
- [x] `internal/cache/cache.go` — `Save` prepends `t.Workdir` to each output pattern; added `root` field to `Cache` struct so `projectRoot()` is always correct (previously returned "" for HybridBackend)
- [x] `internal/cache/cache_test.go` — Added `TestHashTask_WithWorkdir` and `TestCache_SaveAndRestore_WithWorkdir`

**Remaining fixes (recursive glob):**

**Files to modify:**

```
internal/cache/
├── glob.go              # NEW: recursive glob utility
├── glob_test.go         # NEW: tests for recursive glob
├── hasher.go            # Use RecursiveGlob in HashFiles
├── store.go             # Use RecursiveGlob in Save, handle directories in copyFile
└── cache_test.go        # Add tests for ** patterns
internal/cache/cloud/
└── tar.go               # Use RecursiveGlob in createArchive
```

**Step 1: Implement `RecursiveGlob` utility**

**File:** `internal/cache/glob.go` (NEW)

Implement a glob function that treats `**` as recursive descent:

- Split pattern at `**` segments
- For the prefix before `**`, use `filepath.Glob` normally
- For matched directories, use `filepath.Walk` to recurse
- For the suffix after `**`, filter walked files by `filepath.Match`
- Return deduplicated, sorted list of matching file paths

```go
package cache

// RecursiveGlob expands glob patterns with ** support.
// Unlike filepath.Glob, ** matches zero or more directory levels.
// Examples:
//   "src/**/*.go"     → all .go files under src/ at any depth
//   "node_modules/**" → all files under node_modules/ at any depth
//   "dist/*"          → single-level glob (delegated to filepath.Glob)
func RecursiveGlob(root string, pattern string) ([]string, error)
```

**Step 2: Update `HashFiles` to use `RecursiveGlob`**

**File:** `internal/cache/hasher.go`

Replace `filepath.Glob(filepath.Join(root, pattern))` with `RecursiveGlob(root, pattern)` so that `inputs = ["src/**/*.go"]` correctly hashes all nested Go files.

**Step 3: Update `Store.Save` to handle directories**

**File:** `internal/cache/store.go`

Two changes:

1. Replace `filepath.Glob` with `RecursiveGlob` for output pattern matching
2. Since `RecursiveGlob` returns individual files (not directories), `copyFile` will always receive files — no directory handling needed

**Step 4: Update `createArchive` to use `RecursiveGlob`**

**File:** `internal/cache/cloud/tar.go`

Replace `filepath.Glob` with the `RecursiveGlob` utility. Since `RecursiveGlob` returns individual files, the `addDirToTar` path becomes unnecessary — each matched file is added directly via `addFileToTar`.

**Step 5: Tests**

**File:** `internal/cache/glob_test.go` (NEW)

Test cases:

- `**/*.go` matches nested `.go` files
- `node_modules/**` matches all files at any depth
- `dist/*` still works as single-level glob
- `**` alone matches everything recursively
- Empty matches return nil/empty
- Pattern without `**` delegates to `filepath.Glob`

**File:** `internal/cache/cache_test.go`

- Test `Store.Save` and `Store.Restore` with `node_modules/**` pattern and nested files
- Test `Cache.Save` with workdir + recursive patterns
- Test `HashFiles` with `**/*.go` and nested directory structure

**Verification:**

1. `go build ./...` compiles
2. `go test ./internal/cache/...` passes (existing + new tests)
3. `go test ./internal/cache/cloud/...` passes
4. Manual: `dagryn run` with `outputs = ["node_modules/**"]` correctly caches and restores the full tree
5. Remote execution: tasks with `workdir` + `**` patterns restore correctly on fresh clone

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
name: "Dagryn"
description: "Run Dagryn workflows in GitHub Actions"
author: "Dagryn"

branding:
  icon: "zap"
  color: "purple"

inputs:
  targets:
    description: "Tasks or workflows to run"
    required: false
    default: ""
  config:
    description: "Path to dagryn.toml"
    required: false
    default: "dagryn.toml"
  version:
    description: "Dagryn CLI version"
    required: false
    default: "latest"
  remote-cache:
    description: "Enable remote cache"
    required: false
    default: "true"
  cloud-token:
    description: "Dagryn Cloud API token"
    required: false
  project-id:
    description: "Dagryn Cloud project ID"
    required: false
  working-directory:
    description: "Working directory"
    required: false
    default: "."

outputs:
  run-id:
    description: "The Dagryn run ID"
  status:
    description: "The run status (success, failed, cancelled)"
  duration:
    description: "Total run duration in seconds"
  cache-hits:
    description: "Number of cache hits"

runs:
  using: "node20"
  main: "dist/index.js"
```

**Tasks:**

- [x] Create action definition — `github-action/action.yml` (9 inputs, 2 outputs, Node20 runtime)
- [x] Implement CLI installer (with version pinning) — `github-action/src/installer.ts` (resolves latest via GitHub API, platform/arch mapping, tool-cache)
- [x] Implement runner with proper error handling — `github-action/src/runner.ts` (builds args from inputs, captures exit code + duration)
- [x] Implement input parsing — `github-action/src/inputs.ts` (typed `ActionInputs` interface, validation)
- [x] Implement job summary — `github-action/src/summary.ts` (status table, configuration details)
- [x] Implement entry point — `github-action/src/main.ts` (install → run → summary → outputs)
- [x] Bundle with ncc — `github-action/dist/index.js` (single-file bundle ~1MB)
- [x] Create CI workflow — `.github/workflows/ci.yml` (build-action, test, lint, vet jobs)
- [x] Add support for matrix builds — `matrix-label` and `matrix-context` inputs; `DAGRYN_MATRIX_*` env vars; matrix section in job summary
- [x] Add support for PR comments with results — implemented in `notifyGitHub()`: creates/updates PR summary comments via GitHub API with status icon, PR title, commit, branch, SHA, duration, and link to Dagryn run; persists `GitHubPRCommentID` for idempotent updates
- [x] Add support for GitHub commit status updates — implemented via GitHub Check Runs API (`notification.CreateCheckRun`/`UpdateCheckRun`); maps run status to check state/conclusion; builds structured output with task counts and duration; persists `GitHubCheckRunID`

> **Implementation Note:** The action lives in `github-action/` within the main repo (not a separate `dagryn/action` repo). It's a TypeScript/Node20 action built with `@vercel/ncc`. The installer downloads dagryn binaries from GitHub Releases matching GoReleaser naming conventions (e.g., `dagryn_Linux_x86_64.tar.gz`). Supports Linux, macOS, and Windows on x64/arm64. The CI workflow runs 4 independent jobs: action build verification, Go tests with `-race`, golangci-lint, and `go vet`.

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

- [x] Create GitHub App configuration
- [x] Implement installation webhook handler
- [x] Implement push/PR webhook handlers
- [x] Implement Check Runs API integration
- [x] Add PR comment reporting
- [x] Add workflow file generator

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

- [x] Implement runner service
- [x] Add container-based isolation
- [x] Implement workspace management
- [x] Add artifact storage
- [x] Implement log streaming
- [x] Add runner scaling (multiple workers)

---

# Part 3: Dagryn Plugin System

## 3.1 Goal

Create a plugin ecosystem following the **GitHub Actions model** where:

- Plugins are **public GitHub repositories** with a standard structure
- Users reference plugins as `owner/repo@version`
- Dagryn fetches plugin binaries from **GitHub Releases**
- No central registry needed initially (GitHub IS the registry)
- Later: Add optional Dagryn registry for discovery and verified plugins

## 3.2 Architecture (GitHub-First Approach)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    GITHUB-HOSTED PLUGIN SYSTEM                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Plugin Author                          Plugin User                     │
│  ┌────────────────────┐                 ┌────────────────────────────┐ │
│  │ 1. Create repo     │                 │ dagryn.toml                │ │
│  │    dagryn/eslint   │                 │ ┌──────────────────────┐   │ │
│  │                    │                 │ │ [plugins]            │   │ │
│  │ 2. Add plugin.toml │                 │ │ eslint = "dagryn/    │   │ │
│  │                    │                 │ │   eslint@v1.0.0"     │   │ │
│  │ 3. Build binaries  │                 │ └──────────────────────┘   │ │
│  │    for platforms   │                 └────────────┬───────────────┘ │
│  │                    │                              │                 │
│  │ 4. Create GitHub   │                              ▼                 │
│  │    Release with    │                 ┌────────────────────────────┐ │
│  │    binary assets   │                 │ dagryn run build           │ │
│  └─────────┬──────────┘                 └────────────┬───────────────┘ │
│            │                                         │                 │
│            ▼                                         ▼                 │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                         GITHUB                                   │   │
│  │  ┌─────────────────────────────────────────────────────────┐    │   │
│  │  │  Repository: dagryn/eslint                              │    │   │
│  │  │  ├── plugin.toml (manifest)                             │    │   │
│  │  │  ├── README.md                                          │    │   │
│  │  │  └── Releases/                                          │    │   │
│  │  │      └── v1.0.0/                                        │    │   │
│  │  │          ├── eslint-darwin-amd64.tar.gz                 │    │   │
│  │  │          ├── eslint-darwin-arm64.tar.gz                 │    │   │
│  │  │          ├── eslint-linux-amd64.tar.gz                  │    │   │
│  │  │          └── eslint-windows-amd64.zip                   │    │   │
│  │  └─────────────────────────────────────────────────────────┘    │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  Resolution Flow:                                                       │
│  1. Parse "dagryn/eslint@v1.0.0"                                       │
│  2. Fetch plugin.toml from repo (raw.githubusercontent.com)            │
│  3. Download binary from GitHub Release for current platform           │
│  4. Extract and cache in .dagryn/plugins/                              │
└─────────────────────────────────────────────────────────────────────────┘
```

## 3.3 Plugin Types

### Type 1: Tool Plugins (Primary Focus)

Executable binaries that tasks can use. Similar to current implementation but with GitHub hosting.

```toml
# dagryn.toml - Using a tool plugin
[plugins]
eslint = "dagryn/eslint@v1.0.0"
prettier = "prettier/prettier-cli@v3.0.0"

[tasks.lint]
command = "eslint src/"
uses = ["eslint"]
```

### Type 2: Composite Action Plugins (Like GitHub Actions)

Reusable workflows defined in TOML, executed by Dagryn.

```toml
# In plugin repo: dagryn/setup-node/plugin.toml
[plugin]
name = "setup-node"
type = "composite"

[inputs]
node-version = { required = true, description = "Node.js version" }

# Composite plugins define tasks that run in sequence
[[steps]]
name = "Install Node.js"
command = "curl -fsSL https://fnm.vercel.app/install | bash && fnm install ${inputs.node-version}"

[[steps]]
name = "Verify installation"
command = "node --version"
```

```toml
# dagryn.toml - Using a composite plugin
[tasks.setup]
uses = "dagryn/setup-node@v1"
with = { node-version = "20" }

[tasks.build]
needs = ["setup"]
command = "npm run build"
```

### Type 3: Integration Plugins ✅

Extend Dagryn's capabilities via lifecycle hooks (on_run_start, on_run_end, on_task_start, on_task_end, on_run_success, on_run_failure). Implemented in `internal/plugin/hooks.go` (HookExecutor with condition evaluation, variable substitution, 30s timeout), `internal/plugin/integration.go` (IntegrationRegistry with dispatch), manifest validation for integration type, and slack-notify-integration example plugin.

## 3.4 Implementation Phases

### Phase 3.4.1: Plugin Specification (GitHub-Hosted)

**Goal:** Define the plugin structure for GitHub-hosted plugins.

**Repository structure:**

```
dagryn/eslint/
├── plugin.toml          # Plugin manifest (REQUIRED)
├── README.md            # Documentation
├── LICENSE
├── .github/
│   └── workflows/
│       └── release.yml  # Auto-build and release
└── src/                 # Source code (if building from source)
```

**Plugin manifest (plugin.toml):**

```toml
[plugin]
name = "eslint"
description = "ESLint wrapper for Dagryn"
version = "1.0.0"  # Also tagged in git
author = "Dagryn Team <team@dagryn.dev>"
license = "MIT"
homepage = "https://github.com/dagryn/eslint"

# Plugin type: "tool" or "composite"
type = "tool"

# For tool plugins: binary naming convention
# Binaries are downloaded from GitHub Release assets
# Asset names follow: {name}-{os}-{arch}.{ext}
[tool]
binary = "eslint"  # Binary name after extraction

# Supported platforms (Dagryn will download the right one)
# Format: os-arch = "asset-filename-in-release"
[platforms]
darwin-amd64 = "eslint-darwin-amd64.tar.gz"
darwin-arm64 = "eslint-darwin-arm64.tar.gz"
linux-amd64 = "eslint-linux-amd64.tar.gz"
linux-arm64 = "eslint-linux-arm64.tar.gz"
windows-amd64 = "eslint-windows-amd64.zip"

# Optional: runtime dependencies (other plugins)
[dependencies]
# node = "dagryn/node@>=18.0.0"  # Future feature

# For composite plugins: define steps
# [[steps]]
# name = "Step 1"
# command = "echo hello"
```

**Example GitHub Release workflow (.github/workflows/release.yml):**

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

jobs:
  build:
    strategy:
      matrix:
        include:
          - os: ubuntu-latest
            goos: linux
            goarch: amd64
          - os: ubuntu-latest
            goos: linux
            goarch: arm64
          - os: macos-latest
            goos: darwin
            goarch: amd64
          - os: macos-latest
            goos: darwin
            goarch: arm64
          - os: windows-latest
            goos: windows
            goarch: amd64

    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4

      - name: Build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
        run: |
          go build -o eslint-${{ matrix.goos }}-${{ matrix.goarch }}${{ matrix.goos == 'windows' && '.exe' || '' }}

      - name: Package
        run: |
          if [ "${{ matrix.goos }}" = "windows" ]; then
            zip eslint-${{ matrix.goos }}-${{ matrix.goarch }}.zip eslint-${{ matrix.goos }}-${{ matrix.goarch }}.exe
          else
            tar -czvf eslint-${{ matrix.goos }}-${{ matrix.goarch }}.tar.gz eslint-${{ matrix.goos }}-${{ matrix.goarch }}
          fi

      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: eslint-${{ matrix.goos }}-${{ matrix.goarch }}
          path: eslint-*

  release:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Download all artifacts
        uses: actions/download-artifact@v4

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            eslint-*/eslint-*
```

**Tasks:**

- [x] Define plugin.toml specification — `internal/plugin/manifest.go` (`Manifest`, `ManifestPlugin`, `ManifestTool`, `InputDef`, `OutputDef`, `CompositeStep` structs)
- [x] Implement manifest parser — `internal/plugin/manifest.go` (`ParseManifest()` using BurntSushi/toml, `ValidateManifest()`, `IsComposite()`, `IsTool()`, `PlatformAsset()`)
- [x] Document repository structure — template scaffolded by `dagryn plugin init`
- [x] Create release workflow template — generated by `dagryn plugin init <name>` in `.github/workflows/release.yml`
- [x] Write plugin manifest tests — `internal/plugin/manifest_test.go` (12 tests: tool/composite parsing, validation, platform asset lookup)

> **Implementation Note:** The `Manifest` struct maps directly to `plugin.toml` TOML structure. Composite steps use `[[step]]` TOML array of tables. `ValidateManifest()` checks: name required, version required, composite must have steps with commands, type must be "tool"/"composite"/empty. `PlatformAsset()` supports case-insensitive platform key lookup.

### Phase 3.4.2: GitHub Plugin Resolver

**Goal:** Update plugin manager to resolve plugins from GitHub.

**Plugin reference format:**

```toml
[plugins]
# GitHub-hosted plugins (new default)
eslint = "dagryn/eslint@v1.0.0"           # Specific version
prettier = "owner/prettier@v2"             # Major version (latest v2.x.x)
mycompany-lint = "mycompany/lint@latest"   # Latest release

# Explicit GitHub prefix (optional, for clarity)
eslint = "github:dagryn/eslint@v1.0.0"

# Legacy package manager plugins (still supported)
golangci-lint = "go:github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
prettier = "npm:prettier@3.0.0"

# Local development
my-plugin = "local:./plugins/my-plugin"
```

**Resolution flow:**

```go
// internal/plugin/github_resolver.go

type GitHubResolver struct {
    client    *github.Client
    cache     *cache.Cache
    rateLimit *RateLimiter
}

func (r *GitHubResolver) Resolve(ctx context.Context, spec string) (*Plugin, error) {
    // 1. Parse spec: "owner/repo@version"
    owner, repo, version := parseGitHubSpec(spec)

    // 2. Resolve version (handle @latest, @v1, @v1.0.0)
    release, err := r.resolveRelease(ctx, owner, repo, version)
    if err != nil {
        return nil, err
    }

    // 3. Fetch plugin.toml from repo at that tag
    manifest, err := r.fetchManifest(ctx, owner, repo, release.TagName)
    if err != nil {
        return nil, err
    }

    // 4. Find the right asset for current platform
    platform := runtime.GOOS + "-" + runtime.GOARCH
    assetName := manifest.Platforms[platform]
    if assetName == "" {
        return nil, fmt.Errorf("plugin %s/%s does not support %s", owner, repo, platform)
    }

    // 5. Find asset in release
    var downloadURL string
    for _, asset := range release.Assets {
        if asset.GetName() == assetName {
            downloadURL = asset.GetBrowserDownloadURL()
            break
        }
    }

    return &Plugin{
        Name:        manifest.Name,
        Source:      SourceGitHub,
        Owner:       owner,
        Repo:        repo,
        Version:     release.GetTagName(),
        DownloadURL: downloadURL,
        BinaryName:  manifest.Tool.Binary,
    }, nil
}

func (r *GitHubResolver) Install(ctx context.Context, plugin *Plugin, destDir string) (*InstallResult, error) {
    // 1. Download asset
    resp, err := http.Get(plugin.DownloadURL)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // 2. Extract based on extension (.tar.gz, .zip)
    if strings.HasSuffix(plugin.DownloadURL, ".tar.gz") {
        err = extractTarGz(resp.Body, destDir)
    } else if strings.HasSuffix(plugin.DownloadURL, ".zip") {
        err = extractZip(resp.Body, destDir)
    }

    // 3. Find binary and make executable
    binaryPath := filepath.Join(destDir, plugin.BinaryName)
    if runtime.GOOS != "windows" {
        os.Chmod(binaryPath, 0755)
    }

    return &InstallResult{
        Plugin:  plugin,
        Status:  StatusInstalled,
        Message: fmt.Sprintf("Installed %s@%s", plugin.Name, plugin.Version),
    }, nil
}
```

**Files to modify/create:**

```
internal/plugin/
├── plugin.go           # Add SourceGitHubPlugin type
├── github_resolver.go  # NEW: GitHub-based resolution
├── manifest.go         # NEW: Parse plugin.toml
├── version.go          # NEW: Semver resolution
├── resolver.go         # Update resolver registry
└── manager.go          # Update to use new resolver
```

**Tasks:**

- [x] Add short reference format `owner/repo@version` — `internal/plugin/plugin.go` (`shortRefPattern` regex, `parseShortFormat()`, backward-compatible with existing `source:name@version`)
- [x] Add `Manifest` field to `Plugin` struct — `internal/plugin/plugin.go`
- [x] Implement manifest-aware GitHub resolver — `internal/plugin/github.go` (`fetchManifest()` fetches `plugin.toml` from `raw.githubusercontent.com/{owner}/{repo}/{tag}/plugin.toml`)
- [x] Implement deterministic asset selection from manifest — `internal/plugin/github.go` (`findAssetFromManifest()` uses `manifest.Platforms` map, falls back to heuristic scoring)
- [x] Update binary name from manifest — `Resolve()` sets `BinaryName` from `manifest.Tool.Binary` when available
- [x] Add short format tests — `internal/plugin/plugin_test.go` (3 short format cases + owner/precedence tests)
- [x] Add GitHub API rate limiting — `doRequest()` with retry/backoff on 429, `rateLimitState` tracking, auto-detect `DAGRYN_GITHUB_TOKEN`/`GITHUB_TOKEN` env vars
- [x] Add caching for manifest and releases — `DiskCache` with TTL-based Get/Set; 1h for latest/list releases, 24h for tagged releases and manifests; `DAGRYN_NO_PLUGIN_CACHE=1` to disable
- [x] Support GitHub token for private repos — already supported via `WithGitHubToken()` option

> **Implementation Note:** The `Parse()` function tries the long format (`source:name@version`) first, then falls back to the short format (`owner/repo@version`) which defaults to GitHub source. This is fully backward-compatible. The GitHub resolver's `Resolve()` now fetches `plugin.toml` after version resolution — if found, the manifest is stored on `Plugin.Manifest` and used for deterministic asset selection in `Install()`. If no manifest exists or no platform mapping matches, it falls back to the existing heuristic scoring algorithm.

### Phase 3.4.3: Composite Action Plugins

**Goal:** Support composite plugins (like GitHub Actions composite actions).

**Composite plugin example (dagryn/setup-go/plugin.toml):**

```toml
[plugin]
name = "setup-go"
description = "Set up Go environment"
type = "composite"
version = "1.0.0"

[inputs]
go-version = { required = true, description = "Go version to install", default = "1.21" }
cache = { required = false, description = "Enable module cache", default = "true" }

[outputs]
go-path = { description = "Path to Go installation" }

[[steps]]
name = "Download Go"
command = """
curl -LO https://go.dev/dl/go${inputs.go-version}.${os}-${arch}.tar.gz
sudo tar -C /usr/local -xzf go${inputs.go-version}.${os}-${arch}.tar.gz
"""

[[steps]]
name = "Setup PATH"
command = 'echo "export PATH=$PATH:/usr/local/go/bin" >> $HOME/.bashrc'
env = { GOPATH = "${HOME}/go" }

[[steps]]
name = "Verify"
command = "go version"
```

**Usage in dagryn.toml:**

```toml
[tasks.setup-go]
uses = "dagryn/setup-go@v1"
with = { go-version = "1.22", cache = "true" }

[tasks.build]
needs = ["setup-go"]
command = "go build ./..."
```

**Implementation:**

```go
// internal/plugin/composite.go

type CompositePlugin struct {
    Name    string
    Steps   []CompositeStep
    Inputs  map[string]InputDef
    Outputs map[string]OutputDef
}

type CompositeStep struct {
    Name    string
    Command string
    Env     map[string]string
    If      string  // Conditional execution
}

func (p *CompositePlugin) Execute(ctx context.Context, inputs map[string]string, env map[string]string) error {
    for _, step := range p.Steps {
        // Substitute ${inputs.xxx} and ${env.xxx}
        cmd := substituteVars(step.Command, inputs, env)

        // Execute step
        if err := runCommand(ctx, cmd, step.Env); err != nil {
            return fmt.Errorf("step %q failed: %w", step.Name, err)
        }
    }
    return nil
}
```

**Tasks:**

- [x] Define composite plugin schema — `internal/plugin/manifest.go` (`CompositeStep` with Name, Command, If, Env; `Manifest.Steps`)
- [x] Implement variable substitution — `internal/plugin/composite.go` (`substituteVars()` handles `${inputs.key}`, `${os}`, `${arch}`)
- [x] Implement step execution — `internal/plugin/composite.go` (`CompositeExecutor.Execute()` runs steps sequentially via `sh -c`, with input validation and default merging)
- [x] Add conditional steps (if:) — `composite.go` (`step.If` evaluated after variable substitution, skipped when "false" or "")
- [x] Integrate with task scheduler — `internal/scheduler/scheduler.go` (`executeCompositeTask()` detects composite tasks, `compositeExecutor` field, skip binary install for composite plugins in `installPluginsForPlan()`)
- [x] Add `With` field to config and task — `internal/config/schema.go` (`TaskConfig.With`), `internal/task/task.go` (`Task.With`, `IsComposite()`)
- [x] Relax validation for composite tasks — `internal/config/validator.go` (command OR uses required), `internal/task/task.go` (command OR uses required, with requires uses)
- [x] Write composite executor tests — `internal/plugin/composite_test.go` (variable substitution, input merging, step execution, failures, conditionals, env injection)
- [x] Update task tests — `internal/task/task_test.go` (IsComposite, Clone With, uses-without-command, with-without-uses)

> **Implementation Note:** The `CompositeExecutor` validates required inputs, applies defaults from the manifest, substitutes `${inputs.*}`/`${os}`/`${arch}` variables in commands and env values, then executes each step via `sh -c`. The scheduler detects composite tasks (`Command=="" && len(Uses)==1`) and delegates to `executeCompositeTask()` which resolves the plugin, gets its manifest, and calls `CompositeExecutor.Execute()`. In `installPluginsForPlan()`, composite plugins are resolved (manifest fetched) but no binary is downloaded — they are registered in the manager's installed cache via `Manager.Register()` for lock file persistence. The `With` field flows from `dagryn.toml` → `TaskConfig.With` → `task.Task.With` → `CompositeExecutor.Execute()` inputs.
>
> **Composite Setup for Tasks with Commands:** Tasks that have both `command` and `uses` (e.g., `npm install` using `setup-node`) are NOT pure composite tasks — they go through the normal execution path. The scheduler's `runCompositeSetup()` method handles this: before executing the task command, it iterates over `t.Uses`, resolves each plugin, and if it's composite, executes its steps AND collects environment variables via `CollectStepEnv()`. The collected env vars (e.g., `PATH`, `NODE_HOME`) are passed to the executor via `WithExtraEnv()`, which merges them before task-level env vars (task env takes precedence). This ensures composite setup steps (like installing Node.js) take effect for the task command.

### Phase 3.4.4: Plugin CLI Commands

**Goal:** Add CLI commands for plugin management.

**Commands:**

```bash
# List installed plugins
dagryn plugin list

# Show plugin info (fetches from GitHub)
dagryn plugin info dagryn/eslint

# Install a plugin explicitly (usually automatic)
dagryn plugin install dagryn/eslint@v1.0.0

# Update plugins to latest versions
dagryn plugin update

# Remove an installed plugin
dagryn plugin remove eslint

# Create a new plugin from template
dagryn plugin init my-plugin

# Validate plugin.toml
dagryn plugin validate
```

**Tasks:**

- [x] Implement `plugin list` — already existed, no changes needed
- [x] Implement `plugin info` — `internal/cli/plugin.go` (`newPluginInfoCmd()`: resolves `owner/repo@latest`, fetches manifest, displays name/description/type/platforms/inputs/steps)
- [x] Implement `plugin install` — already existed, updated examples to show short format
- [x] Implement `plugin update` — `internal/cli/plugin.go` (`newPluginUpdateCmd()`: loads config, collects plugin specs, re-resolves non-exact versions, re-installs)
- [x] Implement `plugin clean` (remove) — already existed
- [x] Implement `plugin init` (scaffold template) — `internal/cli/plugin.go` (`newPluginInitCmd()`: creates `plugin.toml`, `README.md`, `.github/workflows/release.yml` templates)
- [x] Implement `plugin validate` — `internal/cli/plugin.go` (`newPluginValidateCmd()`: reads `plugin.toml` from cwd, parses + validates, prints summary)
- [x] Add `validatePlugins()` to config validation — `internal/config/validator.go` (validates global plugin specs are parseable)

> **Implementation Note:** All 7 plugin CLI subcommands are registered in `newPluginCmd()`. The `info` command accepts any plugin spec format — it tries `Parse(spec)` directly first, and falls back to appending `@latest` for GitHub shorthand. This supports `local:./path`, `go:pkg@version`, `owner/repo`, etc. The `update` command loads `dagryn.toml`, collects all plugin specs from both global `[plugins]` and task-level `uses`, skips exact versions, cleans and re-installs non-exact versions. The `init` command scaffolds a complete plugin project directory with `plugin.toml`, `README.md`, and a GitHub Actions release workflow. The `validate` command reads `plugin.toml` from the current directory and runs both `ParseManifest()` and `ValidateManifest()`.

### Phase 3.4.5: Official Plugins (dagryn org)

**Goal:** Create official plugins under the `dagryn` GitHub org.

**Initial plugins to create:**

```
dagryn/setup-node       # Install Node.js
dagryn/setup-go         # Install Go
dagryn/setup-python     # Install Python
dagryn/setup-rust       # Install Rust
dagryn/eslint           # ESLint wrapper
dagryn/prettier         # Prettier wrapper
dagryn/golangci-lint    # golangci-lint wrapper
dagryn/pytest           # pytest wrapper
dagryn/jest             # Jest wrapper
dagryn/docker-build     # Docker build helper
dagryn/slack-notify     # Slack notifications
dagryn/cache-s3         # S3 cache backend (integration plugin)
```

**Tasks:**

- [ ] Create dagryn GitHub org — deferred; plugins live locally for now
- [x] Create plugin template repo — `plugin init --type` supports tool/composite/integration; interactive prompt; type-specific plugin.toml, README.md, LICENSE scaffolding
- [x] Create setup-node plugin — `plugins/setup-node/plugin.toml` (downloads Node.js tarball, sets PATH, optional node_modules caching)
- [x] Create setup-go plugin — `plugins/setup-go/plugin.toml` (downloads Go tarball, sets GOROOT+PATH, optional module cache)
- [x] Create setup-python plugin — `plugins/setup-python/plugin.toml` (checks existing version, installs via pyenv/apt/brew, creates virtualenv, optional pip cache)
- [x] Create setup-rust plugin — `plugins/setup-rust/plugin.toml` (installs rustup, installs toolchain, optional cargo cache)
- [x] Create eslint plugin — `plugins/eslint/plugin.toml` (verifies eslint, runs with args/config/fix flags)
- [x] Create prettier plugin — `plugins/prettier/plugin.toml` (runs via npx with --check or --write mode)
- [x] Create golangci-lint plugin — `plugins/golangci-lint/plugin.toml` (installs via curl script, runs with args/config)
- [x] Create pytest plugin — `plugins/pytest/plugin.toml` (verifies pytest, runs with verbose/coverage flags)
- [x] Create jest plugin — `plugins/jest/plugin.toml` (runs via npx with coverage/config flags)
- [x] Create docker-build plugin — `plugins/docker-build/plugin.toml` (builds with tags/build-args, optional push)
- [x] Create slack-notify plugin — `plugins/slack-notify/plugin.toml` (sends POST to webhook URL with JSON payload)
- [x] Create cache-s3 plugin — `plugins/cache-s3/plugin.toml` (conditional restore/save via aws s3 cp)
- [x] Write validation test for all official plugins — `internal/plugin/official_plugins_test.go` (3 test functions: AllValid, RequiredInputs, DefaultInputs — 24 subtests)
- [x] Document all official plugins — `plugins/*/README.md` (13 READMEs with usage, inputs table, examples, notes) + `plugins/*/LICENSE` (MIT license for all 13 plugins)

> **Implementation Note:** Official plugins are implemented as local composite plugins in `plugins/` at the project root (not separate GitHub repos). Each plugin is a single `plugin.toml` file using `type = "composite"` with `[[step]]` entries. All 13 plugins (including slack-notify-integration) share consistent metadata (author: "dagryn", license: "MIT", version: "1.0.0"). The setup-\* plugins download and install toolchains to `$HOME/.dagryn/tools/`, with conditional cache steps gated by `if = "${inputs.cache}"`. Linter/test runner plugins (eslint, prettier, golangci-lint, pytest, jest) wrap existing tools via npx or direct invocation. The `internal/plugin/official_plugins_test.go` test loads all plugins from the `plugins/` directory, parses and validates each manifest, and verifies required inputs, default values, and metadata consistency. Each plugin now has a `README.md` (structured documentation with usage snippets, inputs table, examples) and `LICENSE` (MIT). The backend API serves readme/license content via `PluginInfo.Readme` and `PluginInfo.LicenseText` fields (read from plugin directory). The frontend renders README content via `react-markdown` on plugin detail pages. The `plugin validate` CLI command warns when `README.md` or `LICENSE` files are missing. The registry DB model (`RegistryPlugin`) includes a `readme` column (migration `028_plugin_readme.sql`).

### Phase 3.4.6: Local Plugin System

**Status:** COMPLETE

**Goal:** Support `local:` source type for referencing plugins from the local filesystem (relative or absolute paths). This enables development/testing of plugins without publishing to GitHub and using official plugins bundled with the project.

**Plugin reference format:**

```toml
[plugins]
# Relative to project root
setup-node = "local:./plugins/setup-node"
setup-go = "local:./plugins/setup-go"

# With version pinning (optional, for documentation)
my-plugin = "local:./plugins/my-plugin@1.0.0"

# Absolute path
shared-plugin = "local:/opt/dagryn/shared-plugins/my-tool"
```

**Files created:**

```
internal/plugin/
├── local.go           # LocalResolver implementation
└── local_test.go      # 7 tests for LocalResolver
```

**Tasks:**

- [x] Add `SourceLocal` source type — `internal/plugin/plugin.go` (`SourceLocal SourceType = "local"`, `localPluginPattern` regex)
- [x] Add `parseLocalFormat()` — `internal/plugin/plugin.go` (extracts path and optional version from `local:./path@version`)
- [x] Implement `LocalResolver` — `internal/plugin/local.go` (implements `Resolver` interface: `Resolve()` reads `plugin.toml` from local path, `Install()` sets InstallPath for composite / finds binary for tool, `Verify()` checks manifest exists)
- [x] Register resolver in manager — `internal/plugin/manager.go` (`registry.Register(SourceLocal, NewLocalResolver(projectRoot))`)
- [x] Write local resolver tests — `internal/plugin/local_test.go` (7 tests: Resolve, Resolve_WithExplicitVersion, Resolve_MissingManifest, Resolve_InvalidManifest, Install_Composite, Verify, AbsolutePath)
- [x] Add local plugin parse tests — `internal/plugin/plugin_test.go` (3 cases: relative path, with version, no dot slash)
- [x] Add local plugins to project config — `dagryn.toml` (setup-node, setup-go as `local:./plugins/...`)

> **Implementation Note:** The `LocalResolver` resolves relative paths against `projectRoot` and absolute paths as-is. For composite plugins, `Install()` simply sets `InstallPath` to the resolved directory (no copy needed — the plugin runs from its source location). For tool plugins, it searches for a binary matching `BinaryName` in the plugin directory and `bin/` subdirectory. The `Parse()` function was updated to try the local format first (before long/short GitHub formats) to avoid `local:./path` being misinterpreted as a short reference.

### Phase 3.4.7: Plugin Integration Audit & Fixes

**Status:** COMPLETE

**Goal:** Fix all integration issues discovered during end-to-end testing of the plugin system. These bugs prevented plugins from working correctly across runs, in remote execution, and with the lock file persistence layer.

**Issues fixed:**

1. **Composite plugins not registered in manager** (CRITICAL)
   - **Problem:** `installPluginsForPlan()` in scheduler.go had a `continue` that skipped `Install()` for composite plugins — they were never added to the manager's `installed` map
   - **Fix:** Added `Manager.Register(spec, plugin)` method; scheduler calls it before continuing

2. **Remote execution disables ALL plugins** (CRITICAL)
   - **Problem:** `execute_run.go` had `opts.NoPlugins = true` which disabled plugin resolution for all remote runs
   - **Fix:** Changed to `opts.NoPlugins = false`

3. **Lock file broken for composite/local plugins** (CRITICAL)
   - **Problem:** `loadLockFile()` assumed `BinaryPath` exists; `filepath.Dir(filepath.Dir(""))` returned "." for composite plugins. `saveLockFile()` didn't persist `InstallPath` or `IsComposite`
   - **Fix:** Added `InstallPath` and `IsComposite` fields to `LockFileEntry`. Separate load paths for composite (verify InstallPath + restore Manifest from plugin.toml) vs binary plugins

4. **Manifest lost between runs** (CRITICAL)
   - **Problem:** Lock file serialization doesn't include `Manifest`. On second run, cached plugin has nil Manifest → composite checks fail silently
   - **Fix 1:** `loadLockFile()` reads `plugin.toml` from `InstallPath` for composite entries to restore Manifest
   - **Fix 2:** `Resolve()` falls through to re-resolve via resolver when cached plugin has nil Manifest

5. **findCachedBinary missing SourceLocal** (MAJOR)
   - **Problem:** No `SourceLocal` case in switch — always returned empty string for local tool plugins
   - **Fix:** Added `SourceLocal` case matching binary in plugin dir and `bin/` subdirectory

6. **Plugin info CLI only supports GitHub** (MAJOR)
   - **Problem:** `newPluginInfoCmd()` only accepted GitHub format, had explicit `Source != SourceGitHub` check
   - **Fix:** Try `Parse(spec)` directly first, fall back to GitHub shorthand for backward compatibility

7. **Dead code: ResolveGlobalPlugins** (MINOR)
   - **Problem:** Unused function — config parser has its own `resolvePluginRefs`
   - **Fix:** Removed

8. **executeCompositeTask workdir bug** (MAJOR)
   - **Problem:** `workdir = t.Workdir` used relative path directly instead of joining with projectRoot
   - **Fix:** Changed to `filepath.Join(s.projectRoot, t.Workdir)`

**Files modified:**

- `internal/plugin/manager.go` — Register method, LockFileEntry fields, loadLockFile composite support, saveLockFile fix, findCachedBinary SourceLocal, Install composite caching, Resolve manifest re-resolution, dead code removal
- `internal/scheduler/scheduler.go` — Register call for composites, runCompositeSetup, WithExtraEnv, workdir fix
- `internal/plugin/composite.go` — CollectStepEnv method for env var propagation
- `internal/executor/executor.go` — extraEnv field, WithExtraEnv option, mergedEnv in Execute
- `internal/job/handlers/execute_run.go` — NoPlugins = false
- `internal/cli/plugin.go` — plugin info supports all source types

## 3.5 Future: Dagryn Plugin Registry (Phase 2)

After the GitHub-based system is proven, optionally add a registry layer:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    DAGRYN PLUGIN REGISTRY (FUTURE)                      │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Benefits:                                                              │
│  - Plugin discovery and search                                          │
│  - Verified/official plugin badges                                      │
│  - Download statistics                                                  │
│  - Security scanning                                                    │
│  - Faster downloads (CDN-backed)                                        │
│  - Plugin reviews and ratings                                           │
│                                                                         │
│  How it works:                                                          │
│  - Registry indexes GitHub repos                                        │
│  - Authors "register" their plugin (links GitHub repo)                  │
│  - Registry caches manifests and binaries                               │
│  - Users can still use GitHub directly OR go through registry           │
│                                                                         │
│  Plugin reference:                                                      │
│  - "dagryn/eslint@v1" → Registry (preferred)                           │
│  - "github:owner/repo@v1" → Direct GitHub                              │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

This is deferred to Phase 2 after validating the GitHub-first approach.

---

**Pages (Future):**

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

- [x] Design plugin directory UI
- [x] Implement plugin detail page
- [x] Add search and filtering
- [x] Implement publisher profiles — `publishers/$publisher.tsx` page with stats cards and plugin grid; `plugin_publishers` DB table (migration 027); `GetPublisher`/`CreatePublisher` API endpoints; verified badge support
- [x] Add install code snippets
- [x] Add plugin analytics dashboard — `$publisher.$name.analytics.tsx` with area/bar charts, stat cards, time range selector; `use-plugin-analytics.ts` query hook; `GetRegistryPluginAnalytics` handler; `plugin_downloads` table with daily stats queries
- [x] Add README rendering on plugin detail pages — `react-markdown` on `$pluginName.tsx` and `$publisher.$name.tsx`, license text display on official plugin page
- [x] Add `readme`/`license_text` fields to backend API — `PluginInfo` struct reads README.md/LICENSE from plugin dir; `RegistryPlugin` model has `readme` DB column (migration 028)
- [x] Add `plugin validate` docs warnings — CLI warns on missing README.md/LICENSE

---

# Part 4: Implementation Order

## Recommended Order

```
Phase 1: Foundation ✅ COMPLETE
├── 0.4.1: Storage Interface (pkg/storage/) ✅
├── 0.4.2: Provider Implementations (S3, GCS, Azure, filesystem) ✅
├── 0.4.3: Configuration (env vars, TOML) ✅
├── 0.4.4: Storage Manager ✅
├── 1.3.1: Remote Cache Protocol (internal/cache/remote/) ✅
├── 1.3.2: Cache Backend Abstraction (internal/cache/) ✅
├── 1.3.3: Cache Configuration & CLI ✅
├── 3.4.1: Plugin Specification (plugin.toml manifest) ✅
└── 3.4.2: GitHub Plugin Resolver (short refs, manifest-aware) ✅

Phase 2: Core Features ✅ COMPLETE
├── 2.3.1: GitHub Action Definition ✅
├── 3.4.3: Composite Action Plugins ✅
└── 3.4.4: Plugin CLI Commands (info, update, init, validate) ✅

Phase 3: Integration (IN PROGRESS)
├── 1.3.4: Dagryn Cloud Cache Service ✅
│   ├── Multi-cloud storage (S3, GCS, Azure, R2, MinIO) ✅
│   ├── Cache service + repo + DB migrations ✅
│   ├── HTTP API (check/upload/download/delete/stats/gc/analytics) ✅
│   ├── Background GC job (hourly via asynq) ✅
│   ├── Cache analytics dashboard (backend + frontend) ✅
│   └── Server wiring + env var config ✅
├── 1.3.5: Fix Recursive Glob & Workdir-Aware Caching (IN PROGRESS)
│   ├── Workdir-aware input hashing ✅
│   ├── Workdir-aware output pattern resolution ✅
│   ├── Cache.projectRoot fix for HybridBackend ✅
│   ├── RecursiveGlob utility (** as recursive descent)
│   ├── Update Store.Save to use RecursiveGlob
│   ├── Update createArchive to use RecursiveGlob
│   └── Update HashFiles to use RecursiveGlob
├── 2.3.2: GitHub App Integration ✅
├── 3.4.5: Official Plugins ✅
│   ├── 12 composite plugins in plugins/ directory ✅
│   │   (setup-node, setup-go, setup-python, setup-rust, eslint,
│   │    prettier, golangci-lint, pytest, jest, docker-build,
│   │    slack-notify, cache-s3)
│   └── Validation test suite (24 subtests) ✅
├── 3.4.6: Local Plugin System ✅
│   ├── SourceLocal type + parse format ✅
│   ├── LocalResolver (resolve/install/verify) ✅
│   ├── Manager registration + lock file persistence ✅
│   └── Tests (7 resolver + 3 parse) ✅
├── 3.4.7: Plugin Integration Audit & Fixes ✅
│   ├── Composite plugin registration in manager ✅
│   ├── Remote execution plugin support ✅
│   ├── Lock file composite/local persistence ✅
│   ├── Manifest round-trip restoration ✅
│   ├── Composite setup for tasks with commands ✅
│   ├── Env var propagation (CollectStepEnv → WithExtraEnv) ✅
│   ├── findCachedBinary SourceLocal case ✅
│   ├── Plugin CLI info for all source types ✅
│   ├── executeCompositeTask workdir fix ✅
│   └── Dead code removal ✅
└── Plugin Web Interface (IN PROGRESS)
    ├── Plugin directory UI (browse grid, categories) ✅
    ├── Plugin detail page (metadata, inputs/outputs, steps) ✅
    ├── Search and filtering (client-side search, category tabs) ✅
    ├── Install code snippets (TOML snippet with copy) ✅
    ├── Project plugins page ✅
    ├── API endpoints (list, detail, project plugins) ✅
    ├── Publisher profiles ✅
    ├── Plugin analytics dashboard ✅
    ├── Plugin install/uninstall via API (frontend + routes wired, handlers return 503)
    └── Plugin registry backend ✅

Phase 4: Advanced
├── 2.3.3: Dagryn-Native CI ✅
├── Integration plugin system ✅
└── Task template plugins ✅
```

## Dependencies

```
Remote Cache Protocol ──┬──→ Cache Backend ──→ Cloud Cache Service
         ✅            │         ✅                    ✅
                       └──→ GitHub Action (uses remote cache)
                                  ✅

Plugin Manifest ──→ GitHub Resolver ──┬──→ Composite Plugins ──→ Plugin CLI ──→ Official Plugins
      ✅                  ✅         │         ✅                 ✅              ✅
                                     │                             │
                                     └──→ Local Plugins ✅         ├──→ Integration Audit ✅
                                                                   └──→ Plugin Web UI (partial ✅)

GitHub Action ──→ GitHub App ──→ Native CI
      ✅              ✅            ✅
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
