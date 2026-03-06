# docker-build

Build and optionally push Docker images in your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
docker-build = "local:./plugins/docker-build"

[tasks.build]
uses = "docker-build"
with = { tags = "myapp:latest", context = "." }
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `tags` | **Yes** | | Comma-separated list of image tags (e.g. `myapp:latest,myapp:1.0`) |
| `context` | No | `.` | Docker build context path |
| `file` | No | `Dockerfile` | Path to Dockerfile |
| `push` | No | `false` | Push image to registry after build |
| `build-args` | No | | Comma-separated build args (e.g. `FOO=bar,BAZ=qux`) |
| `cleanup-local-images` | No | `false` | Remove built local images during cleanup |

## Steps

1. **check-docker** - Verifies Docker is available
2. **build-image** - Builds the Docker image with specified tags and build args
3. **push-image** - Pushes images to registry (conditional on `push` input)

## Cleanup

1. **logout-registries** - Logs out of any registries used during push
2. **remove-local-images** - Removes built local images (conditional on `cleanup-local-images`)

## Example

```toml
[tasks.build-and-push]
uses = "docker-build"
with = { tags = "registry.example.com/myapp:latest,registry.example.com/myapp:v1.0", push = "true", build-args = "NODE_ENV=production" }
```

## Notes

- Docker must be installed and running
- Multiple tags can be specified as a comma-separated list
- Build args are passed as `--build-arg` flags
