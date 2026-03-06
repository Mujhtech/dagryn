# setup-go

Install and configure Go for your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
setup-go = "local:./plugins/setup-go"

[tasks.build]
uses = "setup-go"
with = { go-version = "1.22", cache = "true" }
command = "go build ./..."
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `go-version` | No | `1.22` | Go version to install |
| `cache` | No | `true` | Cache Go module cache |

## Steps

1. **download-and-install** - Downloads and installs Go to `~/.dagryn/tools/`
2. **set-environment** - Sets `GOROOT` and adds Go to PATH
3. **cache-modules** - Marks Go module cache for caching

## Example

```toml
[tasks.test]
uses = "setup-go"
with = { go-version = "1.23" }
command = "go test ./..."

[tasks.lint]
uses = "setup-go"
with = { go-version = "1.22" }
command = "golangci-lint run ./..."
```

## Notes

- Go is installed to `~/.dagryn/tools/go-<version>/` and reused across runs
- Module cache at `$GOMODCACHE` is eligible for caching when enabled
