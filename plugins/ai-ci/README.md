# ai-ci

AI-powered failure analysis and inline suggestions for CI runs. This is an **integration** plugin that hooks into Dagryn's run lifecycle events.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
ai-ci = "local:./plugins/ai-ci"
with = { mode = "summarize", backend_mode = "byok" }
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `publish` | No | `github` | Where to publish AI results |
| `mode` | No | `summarize` | AI mode: `summarize` or `summarize_and_suggest` |
| `backend_mode` | No | `byok` | Backend: `managed`, `byok`, or `agent` |

## Hooks

| Hook | Description |
|------|-------------|
| `on_run_failure` | Runs `dagryn ai analyze` when a run fails |

## Example

```toml
[ai]
enabled = true
provider = "openai"
mode = "summarize_and_suggest"

[ai.backend]
mode = "byok"

[ai.backend.byok]
api_key_env = "OPENAI_API_KEY"

[plugins]
ai-ci = "local:./plugins/ai-ci"
with = { mode = "summarize_and_suggest", backend_mode = "byok" }

[tasks.build]
command = "go build ./..."

[tasks.test]
command = "go test ./..."
needs = ["build"]
```

When a run fails, the AI analysis is triggered automatically and prints a summary with root cause, evidence, and recommended actions.

## Notes

- This is an **integration** plugin, not a composite plugin. It uses lifecycle hooks rather than task steps.
- The `$DAGRYN_RUN_ID` and `$DAGRYN_PROJECT_ROOT` environment variables are available in hook commands.
- For local runs, the plugin calls `dagryn ai analyze` which reads logs from `.dagryn/runs/`.
- For server-side (webhook) runs, the AI analysis pipeline is auto-triggered and results are published to GitHub.
- Requires an AI provider API key (e.g. `OPENAI_API_KEY`) when using `byok` backend mode.
