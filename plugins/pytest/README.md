# pytest

Run pytest for Python testing in your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
pytest = "local:./plugins/pytest"

[tasks.test]
uses = "pytest"
with = { verbose = "true", coverage = "false" }
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `args` | No | | Additional arguments to pass to pytest |
| `verbose` | No | `true` | Enable verbose output |
| `coverage` | No | `false` | Enable coverage reporting |

## Steps

1. **check-pytest** - Verifies pytest is installed
2. **run-pytest** - Runs pytest with the specified options

## Example

```toml
[tasks.test]
uses = "pytest"
with = { verbose = "true", coverage = "true" }

[tasks.test-unit]
uses = "pytest"
with = { args = "tests/unit -x", verbose = "true" }
```

## Notes

- pytest must be installed (`pip install pytest`)
- For coverage, install `pytest-cov` (`pip install pytest-cov`)
- Runs via `python3 -m pytest` for reliable module resolution
