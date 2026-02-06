# AGENT_PLAN.md

This document is intended for **code agents or contributors** to get started implementing Dagryn incrementally.

---

## Phase 0 — Ground Rules

Before writing code:

- Do NOT add YAML support
- Do NOT add conditionals or matrices
- Do NOT add containers in v1
- Everything must map cleanly to a DAG

---

## Phase 1 — Core Domain Models

**Goal:** Define stable internal representations.

### Implement

- `Task`
- `Workflow`
- `Graph / DAG`

### Requirements

- Tasks are immutable after parsing
- Dependencies are explicit
- Cycles must be detected and rejected

---

## Phase 2 — Config Parser

**Goal:** Load `dagryn.toml` into domain models.

### Implement

- TOML parser
- Schema validation
- Helpful error messages

### Validation rules

- Missing dependencies → error
- Duplicate task names → error
- Cyclic graph → error

---

## Phase 3 — DAG Builder

**Goal:** Convert tasks into an executable DAG.

### Implement

- Topological sorting
- Dependency resolution
- Ready‑queue generation

### Output

- Ordered execution plan
- Parallelizable task groups

---

## Phase 4 — Task Executor

**Goal:** Execute one task deterministically.

### Implement

- Working directory setup
- Command execution
- Environment injection
- Log capture

### Constraints

- One task = one execution boundary
- Commands must be reproducible

---

## Phase 5 — Cache Engine

**Goal:** Skip work automatically when possible.

### Implement

- Cache key hashing
- Input file hashing
- Output restoration
- Cache metadata

### Rules

- Cache is automatic
- No user‑defined cache keys

---

## Phase 6 — Scheduler

**Goal:** Execute the DAG efficiently.

### Implement

- Dependency tracking
- Parallel execution
- Failure propagation

---

## Phase 7 — CLI

**Goal:** Make Dagryn usable.

### Commands

- `dagryn init`
- `dagryn run <task>`
- `dagryn run <workflow>`
- `dagryn graph`

---

## Phase 8 — Observability (Basic)

**Goal:** Make execution understandable.

### Implement

- Structured logs
- Task timing
- Cache hit/miss reporting

---

## Definition of Done (v1)

Dagryn v1 is complete when:

- A user can define tasks
- Run them locally
- See a DAG
- Get automatic caching
- Debug failures without CI

---

## Final Rule

> If a feature cannot be explained as a deterministic task in a DAG, it does not belong in v1.

---

Based on docs/AGENT_PLAN.md and the current codebase:
| Phase | Goal | Status | Remaining Work |
|-------|------|--------|----------------|
| Phase 0 | Ground Rules | ✅ Done | N/A |
| Phase 1 | Core Domain Models | ✅ Done | internal/task/, internal/dag/ exist |
| Phase 2 | Config Parser | ✅ Done | internal/config/parser.go, schema.go, validator.go exist |
| Phase 3 | DAG Builder | ✅ Done | internal/dag/graph.go, toposort.go, cycle.go exist |
| Phase 4 | Task Executor | ✅ Done | internal/executor/ exists |
| Phase 5 | Cache Engine | ✅ Done | internal/cache/ exists with hasher.go, cache.go, store.go |
| Phase 6 | Scheduler | ✅ Done | internal/scheduler/scheduler.go exists |
| Phase 7 | CLI | ✅ Done | internal/cli/ has init.go, run.go, graph.go |
| Phase 8 | Observability | ✅ Done | internal/telemetry/ exists |

---

Summary
All 8 phases from AGENT_PLAN.md appear to be implemented!
The remaining work is not from the phases but rather code quality/refactoring work:
Remaining Refactoring Tasks (Current Session)

1. ❌ internal/server/handlers/projects.go - Replace WriteJSON/WriteError
2. ❌ internal/server/handlers/runs.go - Replace WriteJSON/WriteError
3. ❌ internal/server/routes.go - Replace handlers.WriteJSON
4. ❌ Build verification (go build ./...)

---

Potential Future Work (Beyond v1 Definition of Done)
The codebase has additional features beyond the v1 scope:

- Server/API layer (internal/server/)
- Dashboard (web UI)
- Multi-user support (projects, teams, invitations)
- Remote run execution
- Plugin system (internal/plugin/)
  These are enhancements beyond the v1 phases defined in AGENT_PLAN.md.
