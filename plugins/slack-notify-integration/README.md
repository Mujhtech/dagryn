# slack-notify-integration

Automatically send Slack notifications on run success or failure. This is an **integration** plugin that hooks into Dagryn's run lifecycle events.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
slack-notify-integration = "local:./plugins/slack-notify-integration"
with = { webhook-url = "${SLACK_WEBHOOK_URL}" }
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `webhook-url` | **Yes** | | Slack incoming webhook URL |

## Hooks

| Hook | Description |
|------|-------------|
| `on_run_success` | Sends a success notification when a run completes |
| `on_run_failure` | Sends a failure notification when a run fails |

## Example

```toml
[plugins]
slack-notify-integration = "local:./plugins/slack-notify-integration"
with = { webhook-url = "${SLACK_WEBHOOK}" }

[tasks.build]
command = "go build ./..."

[tasks.test]
command = "go test ./..."
needs = ["build"]
```

When the pipeline completes, a Slack message is sent automatically.

## Notes

- This is an **integration** plugin, not a composite plugin. It uses lifecycle hooks rather than task steps.
- The `$DAGRYN_RUN_ID` environment variable is available in hook commands
- Requires a Slack incoming webhook URL (create one at https://api.slack.com/messaging/webhooks)
