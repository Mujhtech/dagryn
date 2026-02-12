# setup-python

Install and configure Python for your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
setup-python = "local:./plugins/setup-python"

[tasks.test]
uses = "setup-python"
with = { python-version = "3.12", cache = "true" }
command = "pip install -r requirements.txt && pytest"
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `python-version` | No | `3.12` | Python version to install |
| `cache` | No | `true` | Cache pip packages |
| `cleanup-python-version-file` | No | `false` | Remove `.python-version` after execution (for pyenv local setups) |

## Steps

1. **check-existing** - Checks if the requested Python version is already available
2. **install-python** - Installs Python via pyenv, apt-get, or Homebrew
3. **setup-virtualenv** - Creates a virtualenv in `.venv/`
4. **cache-pip** - Marks pip cache directory for caching

## Example

```toml
[tasks.lint]
uses = "setup-python"
with = { python-version = "3.11" }
command = "pip install flake8 && flake8 src/"
```

## Notes

- Supports installation via pyenv, apt-get, and Homebrew
- Automatically creates a virtualenv at `.venv/` if one doesn't exist
- The `cleanup-python-version-file` option cleans up after pyenv local setups
