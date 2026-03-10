# Telegram Bot Notifier Design

## Overview
The Telegram Bot Notifier provides a rich notification channel for the Red Queen system. It allows the system to send real-time alerts to a Telegram chat (individual or group), including metadata and the captured artifact (image/video).

## Integration Architecture
The Telegram Notifier implements the `notify.Notifier` interface. It is instantiated by the `Coordinator` during startup if configured in `config.yaml`.

### Sequence Diagram
```mermaid
sequenceDiagram
    participant Coordinator
    participant TelegramNotifier
    participant TelegramAPI
    participant User

    Coordinator->>TelegramNotifier: Send(ctx, Event, Result, URL)
    
    rect rgb(240, 240, 240)
    Note over TelegramNotifier: Prepare Message
    TelegramNotifier->>TelegramNotifier: Format MarkdownV2 message with Zone, Confidence, and Labels
    end

    alt Has Local File
        TelegramNotifier->>TelegramAPI: sendPhoto / sendVideo (Multipart Form)
        TelegramAPI-->>User: Alert with Media + Caption
    else File Missing (Fallback)
        TelegramNotifier->>TelegramAPI: sendMessage (JSON)
        TelegramAPI-->>User: Text Alert + Artifact URL
    end

    TelegramAPI-->>TelegramNotifier: HTTP 200 OK
    TelegramNotifier-->>Coordinator: nil (Success)
```

## Configuration
The following fields will be added to the `NotifyConfig` struct in `internal/config/config.go` and supported in `config.yaml`:

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Must be `telegram`. |
| `enabled` | bool | Enables the notifier. |
| `token` | string | The Telegram Bot API token from [@BotFather](https://t.me/botfather). |
| `chat_id` | int64 | The unique identifier for the target chat or group. |
| `url` | string | (Optional) The public base URL of the Red Queen API for artifact links. |

### Example Configuration
```yaml
notifications:
  - type: telegram
    enabled: true
    token: "123456789:ABCdefGHIjklMNOpqrsTUVwxyZ"
    chat_id: -100123456789
    url: "https://my-red-queen.example.com"
```

## Implementation Details

### 1. Message Formatting
The notifier will use `MarkdownV2` for rich text formatting.
- **Header**: 🚨 *Threat Detected\!*
- **Zone**: `event.Zone`
- **Confidence**: `result.Confidence` (formatted as percentage)
- **Labels**: `result.Labels` (comma separated)
- **Link**: A clickable link to the artifact if `url` is configured.

### 2. Media Delivery
Unlike the Webhook or Homey notifiers which only send a URL, the Telegram notifier will attempt to upload the actual file:
- It will read the file from `event.FilePath` (which is guaranteed to exist until the Coordinator's `defer` block executes).
- It will use `sendPhoto` for images and `sendVideo` for other media types.
- The formatted message will be sent as the `caption` of the media.

### 3. Error Handling
- **Rate Limiting**: The notifier will log 429 errors from Telegram but will not block the system.
- **File Access**: If the local file cannot be read, it will fallback to a simple `sendMessage` call with the artifact URL (if available).
- **Network**: Standard Go `http.Client` timeouts (10s) will be applied.

## Security Considerations
- **Token Protection**: The Telegram Bot token allows full control over the bot. It must be kept secret and never checked into source control.
- **Chat ID**: Ensure the bot is added to the group/chat before it can send messages.
