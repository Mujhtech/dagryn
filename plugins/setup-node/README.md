# setup-node

Install and configure Node.js for your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
setup-node = "local:./plugins/setup-node"

[tasks.build]
uses = "setup-node"
with = { node-version = "20", cache = "true" }
command = "npm ci && npm run build"
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `node-version` | No | `20.20.0` | Node.js version to install (full semver) |
| `cache` | No | `true` | Cache node_modules directory |

## Steps

1. **detect-platform** - Detects OS and architecture for the correct Node.js binary
2. **download-and-install** - Downloads and installs Node.js to `~/.dagryn/tools/`
3. **set-path** - Adds Node.js and npm to PATH
4. **cache-node-modules** - Enables caching if a lock file is detected

## Example

```toml
[tasks.lint]
uses = "setup-node"
with = { node-version = "22" }
command = "npx eslint src/"

[tasks.test]
uses = "setup-node"
with = { node-version = "20", cache = "true" }
command = "npm test"
```

## Container Usage

When running in containers that already have Node.js (e.g. `node:20-bookworm-slim`), the plugin detects the pre-installed Node.js and skips the download if the major version matches. This avoids glibc/musl incompatibility issues with Alpine-based containers.

```toml
[tasks.build]
uses = ["setup-node"]
command = "npm run build"
[tasks.build.container]
image = "node:20-bookworm-slim"
```

## Notes

- Node.js is installed to `~/.dagryn/tools/node-<version>/` and reused across runs
- Supports `package-lock.json`, `yarn.lock`, and `pnpm-lock.yaml` for cache detection
- When a matching major version of Node.js is already on PATH, the download step is skipped
