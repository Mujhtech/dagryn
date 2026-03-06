# notify-discord

Send notifications to Discord via webhook.

## Usage

```toml
[tasks.notify]
uses = "dagryn/notify-discord@v1.0.0"
with = { webhook-url = "https://discord.com/api/webhooks/...", message = "Build succeeded!" }
```

### Rich Embed

```toml
[tasks.notify]
uses = "dagryn/notify-discord@v1.0.0"
with = { webhook-url = "https://discord.com/api/webhooks/...", message = "All tests passed", embed-title = "Build Report", embed-color = "3066993" }
```

## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| webhook-url | Yes | - | Discord webhook URL |
| message | Yes | - | Message text to send |
| username | No | Dagryn | Bot username displayed in Discord |
| avatar-url | No | - | Bot avatar URL |
| embed-title | No | - | Embed title for rich message format |
| embed-color | No | - | Embed color as decimal integer (e.g., 3066993 for green) |

## Steps

1. **send-notification** - Send message via Discord webhook (supports simple text and rich embed modes)
