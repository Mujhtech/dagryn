# setup-yarn

Install and configure [Yarn](https://yarnpkg.com/) package manager for your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
setup-yarn = "local:./plugins/setup-yarn"

[tasks.install]
uses = ["setup-node", "setup-yarn"]
with = { yarn-version = "1.22.22", cache = "true" }
command = "yarn install --frozen-lockfile"
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `yarn-version` | No | `1.22.22` | Yarn version to install (classic v1.x) |
| `cache` | No | `true` | Cache Yarn global cache directory |

## Steps

1. **detect-platform** - Detects OS and architecture
2. **download-and-install** - Downloads Yarn from GitHub releases to `~/.dagryn/tools/`
3. **set-path** - Adds Yarn to PATH
4. **cache-yarn** - Enables caching of the Yarn global cache

## Example

```toml
[plugins]
setup-node = "local:./plugins/setup-node"
setup-yarn = "local:./plugins/setup-yarn"

[tasks.install]
uses = ["setup-node", "setup-yarn"]
with = { yarn-version = "1.22.22" }
command = "yarn install --frozen-lockfile"

[tasks.build]
uses = ["setup-node", "setup-yarn"]
needs = ["install"]
command = "yarn build"

[tasks.test]
uses = ["setup-node", "setup-yarn"]
needs = ["build"]
command = "yarn test"
```

## Notes

- Yarn is installed to `~/.dagryn/tools/yarn-<version>/` and reused across runs
- Yarn Classic (v1.x) requires Node.js — pair with `setup-node` or ensure Node.js is on your system PATH
- For Yarn Berry (v2+), consider using `corepack enable` via your Node.js setup instead
- Supports `yarn.lock` for deterministic installs with `--frozen-lockfile`
