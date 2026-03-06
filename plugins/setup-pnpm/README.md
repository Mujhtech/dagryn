# setup-pnpm

Install and configure [pnpm](https://pnpm.io/) package manager for your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
setup-pnpm = "local:./plugins/setup-pnpm"

[tasks.install]
uses = ["setup-node", "setup-pnpm"]
with = { pnpm-version = "10.30.3", cache = "true" }
command = "pnpm install --frozen-lockfile"
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `pnpm-version` | No | `10.30.3` | pnpm version to install (full semver) |
| `node-version` | No | `""` | Node.js version (leave empty to use system Node.js) |
| `cache` | No | `true` | Cache pnpm store directory |

## Steps

1. **detect-platform** - Detects OS and architecture for the correct pnpm binary
2. **download-and-install** - Downloads the standalone pnpm binary to `~/.dagryn/tools/`
3. **set-path** - Adds pnpm to PATH and sets `PNPM_HOME`
4. **cache-pnpm-store** - Enables caching of the pnpm content-addressable store

## Example

```toml
[plugins]
setup-node = "local:./plugins/setup-node"
setup-pnpm = "local:./plugins/setup-pnpm"

[tasks.install]
uses = ["setup-node", "setup-pnpm"]
with = { pnpm-version = "10.30.3" }
command = "pnpm install --frozen-lockfile"

[tasks.build]
uses = ["setup-node", "setup-pnpm"]
needs = ["install"]
command = "pnpm build"

[tasks.test]
uses = ["setup-node", "setup-pnpm"]
needs = ["build"]
command = "pnpm test"
```

## Container Usage

When running in containers that already have Node.js and npm (e.g. `node:20-bookworm-slim`), the plugin installs pnpm via `npm install -g` instead of downloading a standalone binary. This avoids issues with containers that lack `curl`/`wget` and glibc/musl incompatibility with Alpine.

```toml
[tasks.install]
uses = ["setup-node", "setup-pnpm"]
command = "pnpm install --frozen-lockfile"
[tasks.install.container]
image = "node:20-bookworm-slim"
```

## Notes

- pnpm is installed as a standalone binary to `~/.dagryn/tools/pnpm-<version>/` and reused across runs
- pnpm requires Node.js to be available — pair with `setup-node` or ensure Node.js is on your system PATH
- The pnpm store (`~/.local/share/pnpm/store`) is content-addressable and shared across projects
- Supports `pnpm-lock.yaml` for deterministic installs with `--frozen-lockfile`
- When pnpm is already on PATH, the install step is skipped entirely
- When npm is available but pnpm is not, installation uses `npm install -g` instead of standalone binary download
