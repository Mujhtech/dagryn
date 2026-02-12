# upload-artifact

Upload build artifacts to S3-compatible storage.

## Usage

```toml
[tasks.upload]
uses = "dagryn/upload-artifact@v1.0.0"
with = { path = "./dist", name = "build-output", bucket = "my-artifacts" }
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| path | Yes | - | Local path to upload (file or directory) |
| name | Yes | - | Artifact name (used as S3 key prefix) |
| bucket | Yes | - | S3 bucket name |
| region | No | us-east-1 | AWS region |
| endpoint | No | - | Custom S3 endpoint URL (for MinIO, R2, etc.) |
| retention-days | No | 30 | Number of days to retain artifacts |

## Steps

1. **check-aws-cli** - Verify AWS CLI is installed
2. **upload-artifacts** - Upload files to S3 bucket
