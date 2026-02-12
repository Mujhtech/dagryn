# cache-s3

Cache files using Amazon S3 in your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
cache-s3 = "local:./plugins/cache-s3"

[tasks.build]
uses = "cache-s3"
with = { bucket = "my-cache-bucket", key = "deps/node_modules", path = "node_modules" }
command = "npm ci && npm run build"
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `bucket` | **Yes** | | S3 bucket name |
| `key` | **Yes** | | Cache key (S3 object key prefix) |
| `path` | **Yes** | | Local path to cache |
| `region` | No | `us-east-1` | AWS region |
| `restore` | No | `true` | Restore cache from S3 |
| `save` | No | `true` | Save cache to S3 |

## Steps

1. **restore-cache** - Downloads cached files from S3 (conditional on `restore`)
2. **save-cache** - Uploads files to S3 (conditional on `save`)

## Example

```toml
[tasks.build]
uses = "cache-s3"
with = { bucket = "ci-cache", key = "go-mod-${GOVERSION}", path = "$HOME/go/pkg/mod", region = "eu-west-1" }
command = "go build ./..."
```

## Notes

- Requires AWS CLI to be installed and configured with appropriate credentials
- Uses `aws s3 cp --recursive` for file transfer
- Missing cache on restore is not an error (normal for first runs)
