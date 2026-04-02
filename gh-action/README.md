# Dagryn GitHub Action

Run [Dagryn](https://github.com/mujhtech/dagryn) workflows in GitHub Actions. This action installs the Dagryn CLI and executes your DAG-based workflows with full support for caching, parallelism, and matrix builds.

## Usage

### Basic

```yaml
- uses: mujhtech/dagryn@v1
```

### With options

```yaml
- uses: mujhtech/dagryn@v1
  with:
    version: "0.5.0"
    targets: "build test"
    config: "dagryn.toml"
    parallel: 4
    verbose: true
```

### Matrix builds

```yaml
strategy:
  matrix:
    node: [18, 20]
    os: [ubuntu-latest, macos-latest]
steps:
  - uses: actions/checkout@v4
  - uses: mujhtech/dagryn@v1
    with:
      matrix-label: "Node ${{ matrix.node }} / ${{ matrix.os }}"
      matrix-context: '{"node": "${{ matrix.node }}", "os": "${{ matrix.os }}"}'
```

Matrix variables are exposed as `DAGRYN_MATRIX_*` environment variables (e.g. `DAGRYN_MATRIX_NODE`, `DAGRYN_MATRIX_OS`) and can be referenced in your `dagryn.toml` tasks. If `matrix-label` is not set, it is auto-generated from the matrix context values.

### Remote cache

```yaml
- uses: mujhtech/dagryn@v1
  with:
    remote-cache: true
```

### Dry run

```yaml
- uses: mujhtech/dagryn@v1
  with:
    dry-run: true
```

## Inputs

| Input | Description | Default |
|---|---|---|
| `version` | Dagryn CLI version to install | `latest` |
| `targets` | Space-separated task names (empty runs default workflow) | `""` |
| `config` | Path to config file | `dagryn.toml` |
| `parallel` | Max parallel tasks | auto |
| `remote-cache` | Enable remote cache | `false` |
| `no-cache` | Disable all caching | `false` |
| `verbose` | Verbose output | `false` |
| `working-directory` | Working directory | `.` |
| `dry-run` | Show plan without executing | `false` |
| `matrix-label` | Label for this matrix leg (e.g. `Node 20 / Linux`) | `""` |
| `matrix-context` | JSON object of matrix variables passed as `DAGRYN_MATRIX_*` env vars | `""` |

## Outputs

| Output | Description |
|---|---|
| `status` | Run status (`success` or `failure`) |
| `duration` | Total duration in seconds |
| `matrix-label` | The matrix label for this leg (if set) |

## Job Summary

The action writes a GitHub Actions [job summary](https://github.blog/2022-05-09-supercharging-github-actions-with-job-summaries/) with:

- Run status and duration
- Matrix variables (if using matrix builds)
- Configuration details (targets, parallelism, cache settings)

## Supported Platforms

| OS | Architectures |
|---|---|
| Linux | x86_64, arm64 |
| macOS | x86_64, arm64 |
| Windows | x86_64, arm64 |

## License

See the [main repository](https://github.com/mujhtech/dagryn) for license information.
