# Dagryn

**Dagryn** is a local-first, self-hosted developer workflow orchestrator focused on speed, determinism, and great developer experience.

It lets you define workflows as **explicit task graphs** (DAGs) that run the same way locally and in CI — without YAML hell, magic env vars, or slow feedback loops.

> Think: _"CI that feels like running a local command."_

---

## Installation

```bash
# From source
go install github.com/mujhtech/dagryn/cmd/dagryn@latest

# Or build locally
git clone https://github.com/mujhtech/dagryn.git
cd dagryn
make build
./bin/dagryn --help
```

---

## Quick Start

```bash
# Initialize a new project
dagryn init

# Run a specific task
dagryn run build

# Run the default workflow
dagryn run

# Visualize the task DAG
dagryn graph

# Dry run to see execution plan
dagryn run --dry-run test
```

---

## Why Dagryn?

Modern CI tools optimize for scale and security — not for developers.

Dagryn optimizes for:

- **Fast feedback loops** — run locally, skip the push-wait-debug cycle
- **Clear mental models** — explicit DAGs, no hidden magic
- **Easy local debugging** — same behavior locally and in CI
- **Automatic caching** — deterministic cache based on inputs
- **Self-hosting** — own your CI infrastructure

---

## Core Concepts

### Tasks

A **task** is the atomic unit of execution.

Each task:

- has explicit inputs (files/globs)
- produces explicit outputs
- is automatically cached based on inputs
- can depend on other tasks

### Workflows

A **workflow** is a static DAG (directed acyclic graph) of tasks.

Tasks run:

- in parallel where possible
- only when dependencies succeed
- identically in local and CI environments

---

## Configuration

Create a `dagryn.toml` in your project root:

```toml
# dagryn.toml

[workflow]
name = "ci"
default = true  # run all tasks when 'dagryn run' has no arguments

[tasks.install]
command = "npm install"
inputs = ["package.json", "package-lock.json"]
outputs = ["node_modules/**"]
timeout = "5m"

[tasks.build]
command = "npm run build"
needs = ["install"]
inputs = ["src/**", "tsconfig.json"]
outputs = ["dist/**"]
workdir = "./packages/app"  # optional: run in subdirectory

[tasks.test]
command = "npm test"
needs = ["build"]
timeout = "2m"
env = { CI = "true", NODE_ENV = "test" }

[tasks.lint]
command = "npm run lint"
needs = ["install"]
```

### Task Options

| Option    | Description                              | Example                        |
|-----------|------------------------------------------|--------------------------------|
| `command` | Shell command to execute                 | `"npm run build"`              |
| `needs`   | Dependencies (other task names)          | `["install", "setup"]`         |
| `inputs`  | Input file patterns (for caching)        | `["src/**", "*.json"]`         |
| `outputs` | Output file patterns (for caching)       | `["dist/**"]`                  |
| `timeout` | Max execution time                       | `"5m"`, `"30s"`                |
| `workdir` | Working directory (relative to root)     | `"./packages/app"`             |
| `env`     | Environment variables                    | `{ CI = "true" }`              |

---

## CLI Commands

```bash
dagryn init              # Create dagryn.toml template
dagryn run <task>        # Run a task and its dependencies
dagryn run               # Run the default workflow
dagryn graph             # Visualize task DAG (ASCII)
dagryn run --dry-run     # Show execution plan without running
dagryn run --no-cache    # Run without using cache
dagryn run -p 4          # Limit parallel tasks to 4
dagryn run -v            # Verbose output
```

### Global Flags

| Flag           | Description                      |
|----------------|----------------------------------|
| `-c, --config` | Config file (default: dagryn.toml) |
| `-v, --verbose`| Verbose output                   |
| `--no-cache`   | Disable caching                  |

---

## Example Output

```
$ dagryn run test

● install
✓ install      [CACHE HIT] 0.01s
● build
✓ build        [CACHE MISS] 2.34s
● test
✓ test         [CACHE MISS] 1.12s

✓ 3 tasks completed in 3.47s (1 cached)
```

```
$ dagryn graph

Task Dependency Graph
════════════════════════════════════════

┌───────────┐
│  install  │
└───────────┘
     │      
     ▼      
┌───────────┐  ┌───────────┐
│   build   │  │   lint    │
└───────────┘  └───────────┘
     │      
     ▼      
┌───────────┐
│   test    │
└───────────┘

════════════════════════════════════════
Total: 4 tasks
```

---

## Caching

Dagryn automatically caches task outputs based on:

- Task name and command
- Environment variables
- Input file contents (SHA256)

Cache is stored in `.dagryn/cache/` in your project root.

To disable caching:
```bash
dagryn run --no-cache build
```

To clear cache:
```bash
rm -rf .dagryn/cache
```

---

## Design Principles

- **Local = CI** — same behavior everywhere
- **Tasks over steps** — explicit units of work
- **Explicit inputs & outputs** — deterministic caching
- **Deterministic execution** — reproducible builds
- **Convention over configuration** — sensible defaults

---

## Project Structure

```
dagryn/
├── cmd/dagryn/        # CLI entry point
├── internal/
│   ├── task/          # Task and Workflow models
│   ├── dag/           # DAG and topological sorting
│   ├── config/        # TOML parsing and validation
│   ├── executor/      # Task execution
│   ├── cache/         # Caching engine
│   ├── scheduler/     # DAG execution orchestrator
│   └── cli/           # Cobra CLI commands
├── pkg/logger/        # Structured logging
├── testdata/          # Test fixtures
├── dagryn.toml        # Example config
├── go.mod
└── Makefile
```

---

## Roadmap

- [x] Core task engine
- [x] DAG validation & scheduler
- [x] Local cache system
- [x] CLI runner
- [ ] Web UI (DAG + logs)
- [ ] Plugin system (`uses`)
- [ ] Remote cache sharing
- [ ] GitHub Actions integration

---

## Development

```bash
# Build
make build

# Test
make test

# Install locally
make install

# Run
dagryn --help
```

---

## License

MIT
