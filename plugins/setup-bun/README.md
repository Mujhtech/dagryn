# setup-bun

Install and configure [Bun](https://bun.sh/) runtime and package manager for your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
setup-bun = "local:./plugins/setup-bun"

[tasks.install]
uses = ["setup-bun"]
with = { bun-version = "1.2.5", cache = "true" }
command = "bun install --frozen-lockfile"
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `bun-version` | No | `1.2.5` | Bun version to install |
| `cache` | No | `true` | Cache Bun install cache directory |

## Steps

1. **detect-platform** - Detects OS and architecture for the correct Bun binary
2. **download-and-install** - Downloads and extracts Bun to `~/.dagryn/tools/`
3. **set-path** - Adds Bun to PATH and sets `BUN_INSTALL`
4. **cache-bun** - Enables caching of the Bun install cache

## Example

```toml
[plugins]
setup-bun = "local:./plugins/setup-bun"

[tasks.install]
uses = ["setup-bun"]
with = { bun-version = "1.2.5" }
command = "bun install --frozen-lockfile"

[tasks.build]
uses = ["setup-bun"]
needs = ["install"]
command = "bun run build"

[tasks.test]
uses = ["setup-bun"]
needs = ["build"]
command = "bun test"
```

## Notes

- Bun is installed to `~/.dagryn/tools/bun-<version>/` and reused across runs
- Bun is a standalone runtime — it does **not** require Node.js
- Bun includes a built-in package manager, bundler, test runner, and Node.js-compatible runtime
- Supports `bun.lock` for deterministic installs with `--frozen-lockfile`
- Available on macOS (arm64, x64) and Linux (x64, aarch64); Windows support is experimental
