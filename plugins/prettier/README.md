# prettier

Run Prettier for code formatting in your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
prettier = "local:./plugins/prettier"

[tasks.format-check]
uses = "prettier"
with = { args = "src/", check = "true" }
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `args` | No | `.` | Files or directories to format |
| `check` | No | `true` | Check formatting without writing changes |
| `write` | No | `false` | Write formatted output to files |

## Steps

1. **run-prettier** - Runs Prettier with the specified mode and arguments

## Example

```toml
[tasks.format-check]
uses = "prettier"
with = { args = "src/ --ignore-path .gitignore", check = "true" }

[tasks.format-fix]
uses = "prettier"
with = { args = ".", write = "true" }
```

## Notes

- Prettier must be installed in your project (`npm install prettier`)
- Uses `npx prettier` to run
- When both `check` and `write` are set, `write` takes precedence
