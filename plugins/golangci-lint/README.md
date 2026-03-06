# golangci-lint

Run golangci-lint for Go code analysis in your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
golangci-lint = "local:./plugins/golangci-lint"

[tasks.lint]
uses = "golangci-lint"
with = { args = "./..." }
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `version` | No | `latest` | golangci-lint version to install |
| `args` | No | `./...` | Arguments to pass to `golangci-lint run` |
| `config` | No | | Path to golangci-lint config file |

## Steps

1. **install-golangci-lint** - Installs golangci-lint if not already available
2. **run-golangci-lint** - Runs `golangci-lint run` with the specified arguments

## Example

```toml
[tasks.lint]
uses = "golangci-lint"
with = { version = "v1.61.0", args = "./...", config = ".golangci.yml" }
```

## Notes

- Installs golangci-lint via the official install script if not already present
- Requires Go to be available in PATH (use `setup-go` plugin first)
- Uses `go env GOPATH` to determine the install directory
