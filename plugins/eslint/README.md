# eslint

Run ESLint for JavaScript/TypeScript linting in your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
eslint = "local:./plugins/eslint"

[tasks.lint]
uses = "eslint"
with = { args = "src/", fix = "false" }
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `args` | No | `.` | Files or directories to lint |
| `config` | No | | Path to ESLint config file |
| `fix` | No | `false` | Automatically fix problems |

## Steps

1. **check-eslint** - Verifies ESLint is available (via npx or globally)
2. **run-eslint** - Runs ESLint with the specified arguments

## Example

```toml
[tasks.lint-fix]
uses = "eslint"
with = { args = "src/", fix = "true" }

[tasks.lint-check]
uses = "eslint"
with = { args = ".", config = ".eslintrc.json" }
```

## Notes

- ESLint must be installed in your project (`npm install eslint`) or globally
- Uses `npx eslint` to run, so project-local installs are preferred
