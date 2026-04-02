# Dagryn-Native CI: Container Isolation, Artifact Storage, Run Cancellation

## Context

The core CI pipeline already works (webhook -> job queue -> clone -> execute -> report), but three key features are missing for production-grade server-side execution:

1. **Run Cancellation** — Cancel endpoint only updates DB status; running jobs keep executing
2. **Artifact Storage** — No way to collect/store/download build artifacts from server-side runs
3. **Container Isolation** — Tasks execute directly on the host via `sh -c`; no sandboxing

## Implementation Order

| Phase | Feature             | Scope    | Notes                            |
| ----- | ------------------- | -------- | -------------------------------- |
| 1     | Run Cancellation    | Smallest | Highest standalone value         |
| 2     | Artifact Storage    | Moderate | Reuses existing storage patterns |
| 3     | Container Isolation | Largest  | Requires Docker SDK dependency   |

---

## Phase 1: Run Cancellation

### Problem

`CancelRun` handler (`internal/server/handlers/runs.go:900`) only sets DB status to `cancelled`. The actual `execute_run` asynq job continues running. No signal reaches the scheduler/executor.

### Design

Use Redis pub/sub + TTL key for cancellation signals:

1. API handler publishes cancel signal to Redis
2. `ExecuteRunHandler` watches for signal via goroutine
3. Signal triggers `context.Cancel()` which propagates through scheduler -> executor -> `exec.CommandContext`

### Files to Create

#### `internal/job/cancel.go`

CancelManager using Redis pub/sub:

```go
type CancelManager struct {
    rds *redis.Redis
}

func NewCancelManager(rds *redis.Redis) *CancelManager
func (m *CancelManager) Signal(ctx context.Context, runID string) error  // PUBLISH + SET key with 1h TTL
func (m *CancelManager) Watch(ctx context.Context, runID string) <-chan struct{}  // SUBSCRIBE + check existing key
func (m *CancelManager) Clear(ctx context.Context, runID string) error  // DEL key
```

> **Implementation Note**: The cancel key naming should follow the pattern used in `internal/job/type.go` for task name constants (e.g., `cancel:run:{runID}`).

#### `internal/job/cancel_test.go`

Tests for signal/watch/clear operations.

### Files to Modify

#### `internal/job/job.go`

- Add `CancelManager` field to `Job` struct and `Config`
- Create in `New()`
- Pass to `ExecuteRunHandler`

#### `internal/job/handlers/execute_run.go`

Add cancellation watcher:

- Accept `cancelManager` in `NewExecuteRunHandler()`
- In `Handle()`, wrap execution context with cancel func
- Goroutine watches for cancel signal and calls `cancel()`
- After completion, if ctx was cancelled, set run status to `cancelled` **only if** current status is still `running` (avoid overwriting `success` or `failed` on late cancel signals)
- Ensure the cancel watcher unsubscribes/cleans up on completion to avoid goroutine or pub/sub leaks

> **Implementation Note**: The existing `ExecuteRunHandler` (848 lines) already uses callbacks for task lifecycle events (`OnTaskStart`, `OnTaskComplete`, `OnLogLine`). The cancellation watcher should integrate with this callback pattern for consistent status reporting.

#### `internal/server/handlers/handler.go`

- Add `cancelManager *job.CancelManager` field
- Update `New()` signature

#### `internal/server/handlers/runs.go`

In `CancelRun()` (line 900), after DB update, call `cancelManager.Signal()` to propagate to running worker.

#### `internal/server/server.go`

- Create `CancelManager` from Redis
- Pass to both `Handler` and `Job`

### Database Changes

**No DB migration needed** — `cancelled` status already exists.

---

## Phase 2: Artifact Storage

### Problem

No way to collect, store, or download build artifacts from server-side runs. Tasks produce binaries, test reports, coverage files that are lost when the temp workspace is cleaned up.

### Design

Mirror the existing cache storage pattern: metadata in PostgreSQL, blobs in object storage via `pkg/storage.Bucket`. Artifacts scoped to run + optional task.

**Storage key pattern**: `artifacts/{projectId}/{runID}/{taskName}/{artifactID}/{fileName}`

### Database Migration

#### `internal/db/migrations/025_artifacts.sql`

> **Note**: Verified that `023_cache_usage.sql` is the current highest migration.

```sql
CREATE TABLE artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    task_name VARCHAR(255),
    name VARCHAR(512) NOT NULL,
    file_name VARCHAR(512) NOT NULL,
    content_type VARCHAR(255) NOT NULL DEFAULT 'application/octet-stream',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    storage_key VARCHAR(1024) NOT NULL,
    digest_sha256 VARCHAR(64),
    expires_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_artifacts_project ON artifacts(project_id);
CREATE INDEX idx_artifacts_run ON artifacts(run_id);
CREATE INDEX idx_artifacts_run_task ON artifacts(run_id, task_name);
CREATE INDEX idx_artifacts_expires ON artifacts(expires_at) WHERE expires_at IS NOT NULL;
```

### Files to Create

#### `internal/db/models/artifact.go`

Artifact model struct matching the DB schema.

#### `internal/db/repo/artifacts.go`

`ArtifactRepo` with CRUD operations:

- `Create(ctx, artifact) (*Artifact, error)`
- `GetByID(ctx, id) (*Artifact, error)`
- `ListByRun(ctx, runID, limit, offset) ([]*Artifact, error)`
- `ListByRunAndTask(ctx, runID, taskName, limit, offset) ([]*Artifact, error)`
- `Delete(ctx, id) error`
- `DeleteExpired(ctx) (int64, error)`
- `TotalSizeByProject(ctx, projectID) (int64, error)`

#### `internal/db/repo/artifacts_test.go`

Unit tests for artifact repository operations.

#### `internal/service/artifact.go`

`ArtifactService` with business logic:

```go
type ArtifactService struct {
    repo    *repo.ArtifactRepo
    bucket  storage.Bucket
    signer  storage.SignedURLer  // Optional, for pre-signed URLs
}

func (s *ArtifactService) Upload(ctx, projectID, runID, taskName, name, fileName string, reader io.Reader, size int64, ttl time.Duration) (*Artifact, error)
func (s *ArtifactService) Download(ctx, artifactID string) (io.ReadCloser, error)
func (s *ArtifactService) DownloadURL(ctx, artifactID string, expiry time.Duration) (string, error)  // Uses SignedURLer if available
func (s *ArtifactService) List(ctx, runID, taskName string, limit, offset int) ([]*Artifact, error)
func (s *ArtifactService) Delete(ctx, artifactID string) error
func (s *ArtifactService) CleanupExpired(ctx) (int64, error)
```

> **Implementation Note**: The `pkg/storage` package includes a `SignedURLer` interface for pre-signed URL support. Check if the configured bucket implements this interface before offering `DownloadURL`.

#### `internal/service/artifact_test.go`

Unit tests for artifact service operations.

#### `internal/server/handlers/artifacts.go`

HTTP handlers:

| Method | Route                                              | Handler            | Description                      |
| ------ | -------------------------------------------------- | ------------------ | -------------------------------- |
| GET    | `.../runs/{runID}/artifacts`                       | `ListRunArtifacts` | List artifacts for a run         |
| POST   | `.../runs/{runID}/artifacts`                       | `UploadArtifact`   | Upload artifact (multipart form) |
| GET    | `.../runs/{runID}/artifacts/{artifactID}`          | `GetArtifact`      | Get artifact metadata            |
| GET    | `.../runs/{runID}/artifacts/{artifactID}/download` | `DownloadArtifact` | Download artifact content        |
| DELETE | `.../runs/{runID}/artifacts/{artifactID}`          | `DeleteArtifact`   | Delete artifact                  |

> **Security/limits**: Validate the authenticated user has access to the project/run, enforce upload size limits, and reject unexpected content types.

### Files to Modify

#### `internal/server/config.go`

Add `ArtifactStorage StorageConfig` field to `Config`.

> **Implementation Note**: The existing `StorageConfig` struct (lines 29-41) and env var pattern (`DAGRYN_CACHE_STORAGE_*`) should be mirrored exactly for artifacts using `DAGRYN_ARTIFACT_STORAGE_*`.

```go
type Config struct {
    // ... existing fields ...
    CacheStorage    StorageConfig `toml:"cache_storage"`
    ArtifactStorage StorageConfig `toml:"artifact_storage"`  // Add this
}
```

Add environment variable overrides following the existing pattern:

```go
// DAGRYN_ARTIFACT_STORAGE_PROVIDER
// DAGRYN_ARTIFACT_STORAGE_BUCKET
// DAGRYN_ARTIFACT_STORAGE_REGION
// DAGRYN_ARTIFACT_STORAGE_ENDPOINT
// DAGRYN_ARTIFACT_STORAGE_ACCESS_KEY_ID
// DAGRYN_ARTIFACT_STORAGE_SECRET_ACCESS_KEY
// DAGRYN_ARTIFACT_STORAGE_USE_PATH_STYLE
// DAGRYN_ARTIFACT_STORAGE_BASE_PATH
// DAGRYN_ARTIFACT_STORAGE_PREFIX
// DAGRYN_ARTIFACT_STORAGE_CREDENTIALS_FILE
```

#### `internal/server/server.go`

- Initialize artifact storage bucket using `storage.NewBucket()`
- Create `ArtifactService`
- Pass to `Handler`

#### `internal/server/handlers/handler.go`

- Add `artifactService *service.ArtifactService` field
- Update `New()` signature

#### `internal/server/routes.go`

Add artifact routes under `r.Route("/{runID}", ...)`:

```go
r.Route("/artifacts", func(r chi.Router) {
    r.Get("/", h.ListRunArtifacts)
    r.Post("/", h.UploadArtifact)
    r.Route("/{artifactID}", func(r chi.Router) {
        r.Get("/", h.GetArtifact)
        r.Get("/download", h.DownloadArtifact)
        r.Delete("/", h.DeleteArtifact)
    })
})
```

#### `internal/job/type.go`

Add task name constant:

```go
const ArtifactCleanupTaskName = "artifact_cleanup:daily"
```

#### `internal/job/job.go`

- Accept `ArtifactService` in `Config`
- Register artifact cleanup handler on daily schedule in `RegisterAndStart()`

#### `internal/job/handlers/execute_run.go`

After scheduler completes, auto-collect declared outputs as artifacts using `ArtifactService.Upload()`.

---

## Phase 3: Container Isolation

### Problem

Tasks run directly on the host via `exec.CommandContext(ctx, "sh", "-c", command)`. No filesystem isolation, no resource limits, tasks can affect each other.

### Design

Introduce a `TaskExecutor` interface so the scheduler can use either host or container execution transparently. Graceful degradation: if Docker unavailable, warn and fall back to host execution.

### New Dependency

```bash
go get github.com/docker/docker
```

Docker Engine SDK for container management.

### Files to Create

#### `internal/container/runtime.go`

Docker runtime abstraction:

```go
type Runtime interface {
    Available(ctx context.Context) bool
    Pull(ctx context.Context, image string) error
    Create(ctx context.Context, cfg ContainerConfig) (containerID string, err error)
    Start(ctx context.Context, containerID string) error
    Wait(ctx context.Context, containerID string) (exitCode int, err error)
    Logs(ctx context.Context, containerID string, stdout, stderr io.Writer) error
    Stop(ctx context.Context, containerID string, timeout time.Duration) error
    Remove(ctx context.Context, containerID string) error
}

type ContainerConfig struct {
    Image       string
    Command     []string
    WorkDir     string
    Env         map[string]string
    Mounts      []Mount
    CPULimit    int64   // CPU quota in units of 10^-9 CPUs
    MemoryLimit int64   // Memory limit in bytes
    Network     string
}

type Mount struct {
    Source   string
    Target   string
    ReadOnly bool
}
```

#### `internal/container/docker.go`

`DockerRuntime` implementing `Runtime` via Docker SDK.

#### `internal/container/executor.go`

`ContainerExecutor` implementing `executor.TaskExecutor`:

- Mounts `projectRoot` -> `/workspace`
- Creates container per task with resource limits
- Streams logs to stdout/stderr writers
- Cleans up container on completion or cancellation

> **Implementation Note**: Container cleanup (stop + remove) should use `context.Background()` to ensure cleanup completes even when the main execution context is cancelled. This prevents orphaned containers.

```go
func (e *ContainerExecutor) cleanup(containerID string) {
    // Use background context to ensure cleanup completes
    ctx := context.Background()
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    _ = e.runtime.Stop(ctx, containerID, 10*time.Second)
    _ = e.runtime.Remove(ctx, containerID)
}
```

#### `internal/container/config.go`

Configuration types for container isolation.

#### `internal/container/*_test.go`

Unit tests with mock Docker client.

### Files to Modify

#### `internal/executor/executor.go`

Extract `TaskExecutor` interface:

```go
type TaskExecutor interface {
    Execute(ctx context.Context, t *task.Task) *Result
    DryRun(t *task.Task) *Result
}
```

> **Implementation Note**: The existing `*Executor` struct already has these methods and will satisfy this interface. The existing `Result` struct and `Status` enum should be reused by `ContainerExecutor`. This is a backward-compatible change.

#### `internal/task/task.go`

Add optional container configuration:

```go
type Task struct {
    // ... existing fields ...
    Container *TaskContainerConfig  // Optional per-task container settings
}

type TaskContainerConfig struct {
    Image       string
    MemoryLimit string  // e.g., "2g"
    CPULimit    string  // e.g., "2.0"
    Network     string
}
```

#### `internal/config/schema.go`

Add project-level `[container]` config section + per-task `[tasks.X.container]`:

```toml
[container]
enabled = true
image = "golang:1.25"
memory_limit = "2g"
cpu_limit = "2.0"
network = "bridge"

[tasks.build.container]
image = "golang:1.25-alpine"
memory_limit = "4g"
```

#### `internal/config/parser.go`

Propagate container config to `Task` in `taskConfigToTask()`.

#### `internal/scheduler/scheduler.go`

- Accept `TaskExecutor` interface instead of concrete `*executor.Executor`
- Add container config to `Options`
- Create appropriate executor based on config + Docker availability
- Define configuration precedence: server defaults < project config < task-level overrides

```go
type Options struct {
    // ... existing fields ...
    ContainerConfig *container.Config
    TaskExecutor    TaskExecutor  // Interface, not concrete type
}
```

#### `internal/server/config.go`

Add container configuration:

```go
type ContainerServerConfig struct {
    Enabled     bool   `toml:"enabled"`
    DefaultImage string `toml:"default_image"`
    MemoryLimit string `toml:"memory_limit"`
    CPULimit    string `toml:"cpu_limit"`
    Network     string `toml:"network"`
}

type Config struct {
    // ... existing fields ...
    Container ContainerServerConfig `toml:"container"`
}
```

Add environment variable overrides:

```go
// DAGRYN_CONTAINER_ENABLED
// DAGRYN_CONTAINER_DEFAULT_IMAGE
// DAGRYN_CONTAINER_MEMORY_LIMIT
// DAGRYN_CONTAINER_CPU_LIMIT
// DAGRYN_CONTAINER_NETWORK
```

#### `internal/job/handlers/execute_run.go`

- Pass container config from parsed `dagryn.toml` to scheduler
- Pass server-level container defaults
- Merge project config with server defaults

---

## Verification

### Phase 1: Run Cancellation

#### Build Verification

```bash
go build ./...
```

#### Unit Tests

```bash
go test ./internal/job/...
```

#### Integration Test

1. Trigger a long-running server-side run (e.g., `sleep 60`)
2. POST to `/api/v1/projects/{projectId}/runs/{runID}/cancel`
3. Verify the running process is killed
4. Verify run status becomes `cancelled` in DB
5. Verify Redis cancel key is cleared after completion

#### Rollback Procedure

1. Revert changes to `runs.go`, `execute_run.go`, `job.go`, `server.go`, `handler.go`
2. Remove `internal/job/cancel.go` and `internal/job/cancel_test.go`
3. No DB migration to rollback

---

### Phase 2: Artifact Storage

#### Build Verification

```bash
go build ./...
```

#### Unit Tests

```bash
go test ./internal/db/repo/...   # artifact repo
go test ./internal/service/...    # artifact service
```

#### Integration Test

1. POST multipart upload to `/api/v1/projects/{projectId}/runs/{runID}/artifacts`
2. GET list of artifacts for run
3. GET download and verify content integrity (SHA256 match)
4. DELETE artifact and verify removal from both DB and storage
5. Test expired artifact cleanup job

#### Rollback Procedure

1. Run down migration: `DROP TABLE artifacts;`
2. Revert changes to config, server, handlers, routes
3. Remove created files in `db/models`, `db/repo`, `service`, `handlers`

---

### Phase 3: Container Isolation

#### Build Verification

```bash
go build ./...
```

#### Unit Tests

```bash
go test ./internal/container/...  # mock Docker
go test ./internal/executor/...   # interface compatibility
```

#### Integration Test (Docker Available)

1. Set `[container] enabled = true` in `dagryn.toml`
2. Run a task and verify it executes inside a container
3. Verify resource limits are applied (`docker stats`)
4. Verify workspace is mounted correctly
5. Verify container is cleaned up after task completion
6. Test cancellation cleans up container

#### Integration Test (Docker Unavailable)

1. Stop Docker daemon or run on machine without Docker
2. Set `[container] enabled = true`
3. Verify warning is logged
4. Verify graceful fallback to host execution
5. Verify tasks still complete successfully

#### Rollback Procedure

1. Revert `internal/executor/executor.go` interface extraction
2. Revert `internal/scheduler/scheduler.go` to use concrete executor
3. Revert config changes
4. Remove `internal/container/` directory
5. Remove Docker SDK dependency: `go mod tidy`

---

## Dependencies Between Phases

```
Phase 1 (Cancellation) ──┐
                         ├──> Phase 3 (Container Isolation)
Phase 2 (Artifacts) ─────┘
```

- **Phases 1 and 2** are independent and can be implemented in parallel by different developers
- **Phase 3** benefits from Phase 1 (cancellation propagates to container stop) but doesn't strictly require it
- **Phase 3** is independent of Phase 2

## Estimated Effort

| Phase                   | Effort   | Risk                                        |
| ----------------------- | -------- | ------------------------------------------- |
| 1 - Run Cancellation    | 2-3 days | Low - Redis patterns well-established       |
| 2 - Artifact Storage    | 3-4 days | Low - Mirrors existing cache implementation |
| 3 - Container Isolation | 5-7 days | Medium - Docker SDK complexity, edge cases  |

**Total**: ~10-14 days for all three phases
