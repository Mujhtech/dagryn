# setup-rust

Install and configure the Rust toolchain for your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
setup-rust = "local:./plugins/setup-rust"

[tasks.build]
uses = "setup-rust"
with = { rust-version = "stable", cache = "true" }
command = "cargo build --release"
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `rust-version` | No | `stable` | Rust toolchain version to install (stable, nightly, or specific version) |
| `cache` | No | `true` | Cache cargo registry and target directory |

## Steps

1. **install-rustup** - Installs rustup if not already available
2. **install-toolchain** - Installs and sets the requested Rust toolchain as default
3. **verify** - Prints rustc and cargo versions
4. **cache-cargo** - Marks cargo registry and target directory for caching

## Example

```toml
[tasks.test]
uses = "setup-rust"
with = { rust-version = "nightly" }
command = "cargo test"

[tasks.clippy]
uses = "setup-rust"
with = { rust-version = "stable" }
command = "cargo clippy -- -D warnings"
```

## Notes

- Installs rustup automatically if not present
- Supports `stable`, `nightly`, and specific version strings (e.g., `1.75.0`)
- Cargo registry and `target/` directory are eligible for caching
