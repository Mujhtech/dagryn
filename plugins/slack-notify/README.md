# slack-notify

Send notifications to Slack via webhook in your Dagryn tasks.

## Usage

Add to your `dagryn.toml`:

```toml
[plugins]
slack-notify = "local:./plugins/slack-notify"

[tasks.notify]
uses = "slack-notify"
with = { webhook-url = "${SLACK_WEBHOOK_URL}", message = "Build completed successfully" }
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `webhook-url` | **Yes** | | Slack incoming webhook URL |
| `message` | **Yes** | | Message text to send |
| `channel` | No | | Slack channel override |
| `username` | No | `Dagryn` | Bot username displayed in Slack |

## Steps

1. **send-notification** - Sends a JSON payload to the Slack webhook URL

## Example

```toml
[tasks.notify-deploy]
uses = "slack-notify"
with = { webhook-url = "${SLACK_WEBHOOK}", message = "Deployed v1.2.3 to production", channel = "#deployments", username = "Deploy Bot" }
```

## Notes

- Requires a Slack incoming webhook URL (create one at https://api.slack.com/messaging/webhooks)
- Store the webhook URL in an environment variable to avoid exposing it in config
- Uses `curl` to send the notification
