# Plugin system

Dagryn’s plugin system lets tasks declare tool dependencies that are installed and cached automatically for **local** runs.

## Plugin spec format

In `dagryn.toml`, tasks can declare plugins in the format:

```text
source:name@version
```

- **source**: One of `github`, `go`, `npm`, `pip`, `cargo`.
- **name**: Package/repo identifier (e.g. `owner/repo` for GitHub, package name for npm/pip/cargo, module path for Go).
- **version**: Version constraint (e.g. `v1.0.0`, `latest`, `^1.0.0`).

Plugins can be specified as a single string or an array of strings:

```toml
[tasks.lint]
command = "golangci-lint run"
uses = "github:golangci/golangci-lint@v1.55.0"

[tasks.format]
command = "prettier --write ."
uses = ["npm:prettier@3.0.0", "npm:some-other-tool@1.0.0"]
```

## Supported sources

| Source  | Example spec                               | Description                    |
|---------|--------------------------------------------|--------------------------------|
| `github`| `github:owner/repo@v1.0.0`                 | GitHub releases (binary)       |
| `go`    | `go:golang.org/x/tools/cmd/goimports@latest` | Go modules (`go install`)   |
| `npm`   | `npm:prettier@3.0.0` or `npm:@org/pkg@1.0.0` | npm packages                  |
| `pip`   | `pip:black@23.12.0`                        | Python packages (pip)          |
| `cargo` | `cargo:ripgrep@14.0.3`                     | Rust crates (cargo install)    |

- **GitHub**: Resolves releases by tag; downloads the appropriate asset for the current OS/arch.
- **Go**: Runs `go install <module>@<version>` (requires Go toolchain).
- **NPM**: Runs `npm install -g` (or uses a project-local install); adds the package bin to `PATH`.
- **Pip**: Runs `pip install`; adds the environment’s scripts to `PATH`.
- **Cargo**: Runs `cargo install` (requires Rust toolchain).

## How plugins are used (local runs)

1. When you run `dagryn run [tasks]`, the scheduler builds an execution plan from the DAG.
2. For each task in the plan, it collects plugin specs from the task’s `uses` and from the global `[plugins]` section.
3. The plugin **Manager** (see `internal/plugin/manager.go`):
   - Resolves each spec (version resolution, GitHub release URL, etc.) via the **ResolverRegistry**.
   - Installs plugins into `.dagryn/plugins/` under the project root, keyed by source/name/version.
   - Caches installs so the same spec is not reinstalled on every run.
4. The executor runs the task with the plugin binary directories prepended to `PATH`, so the task’s `command` can invoke the tool by name.

So plugins are **only used when running the DAG locally** (e.g. `dagryn run` or `dagryn run --sync`). The scheduler option `NoPlugins` can disable plugin installation for a run.

## Server-side (remote) runs

For **server-triggered runs** (e.g. runs created via the API or webhooks and executed by the worker):

- The ExecuteRun job runs the workflow with **plugins disabled** (`NoPlugins: true`).
- The worker does not install or resolve plugins; task `uses` are ignored.
- Rationale: server runs use an ephemeral clone and a controlled environment. Installing arbitrary tools (npm, pip, cargo, etc.) would require extra toolchains, network, and security policy. To use tools on server runs, they must already be present in the worker environment or be part of the repository (e.g. scripts or vendored binaries).

This is the current **server-side plugin policy**. It may be revisited if worker images and policies are defined to support plugin installation.

## Implementation notes

- **Parsing**: `internal/plugin/plugin.go` defines `Parse(spec)` and `Spec` (TOML unmarshaling).
- **Resolvers**: `internal/plugin/resolver.go` defines the registry; `github.go`, `goinstall.go`, `npm.go`, `pip.go`, `cargo.go` implement per-source resolution and install.
- **Scheduler**: `internal/scheduler/scheduler.go` uses `plugin.NewManager(projectRoot)` and installs plugins for the execution plan when `NoPlugins` is false.
