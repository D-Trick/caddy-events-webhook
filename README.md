# Caddy Module Event Webhook

When an event occurs in Caddy, a webhook request is sent to the specified URL.

## Configuration Options

- `header`: HTTP headers (optional)
- `timeout`: HTTP request timeout in seconds (default: 30s)

## Configuration Examples

```caddyfile
{
    events {
        # on eventName webhook URL
        on cert_obtained webhook https://webhook.com/api/webhook {
            header Content-Type application/json
            header User-Agent Webhook
        }
    }
}
```
