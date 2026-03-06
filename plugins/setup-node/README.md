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
| `node-version` | No | `20` | Node.js version to install |
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

## Notes

- Node.js is installed to `~/.dagryn/tools/node-<version>/` and reused across runs
- Supports `package-lock.json`, `yarn.lock`, and `pnpm-lock.yaml` for cache detection
