# jest

Run Jest for JavaScript/TypeScript testing in your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
jest = "local:./plugins/jest"

[tasks.test]
uses = "jest"
with = { coverage = "false" }
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `args` | No | | Additional arguments to pass to Jest |
| `coverage` | No | `false` | Enable coverage reporting |
| `config` | No | | Path to Jest config file |

## Steps

1. **run-jest** - Runs Jest with the specified options

## Example

```toml
[tasks.test]
uses = "jest"
with = { coverage = "true" }

[tasks.test-unit]
uses = "jest"
with = { args = "--testPathPattern=unit", config = "jest.unit.config.js" }
```

## Notes

- Jest must be installed in your project (`npm install jest`)
- Uses `npx jest` to run
