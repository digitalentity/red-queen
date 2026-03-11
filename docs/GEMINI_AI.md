# Setting up Gemini AI for Red Queen

Red Queen uses Google's **Gemini AI** for multimodal video analysis. This requires an API Key from Google AI Studio.

## Prerequisites

1.  **Google AI Studio Account**: Access [Google AI Studio](https://aistudio.google.com/).
2.  **API Key**: Generate an API Key in AI Studio.
3.  **Model Selection**: By default, Red Queen is configured to use `gemini-1.5-flash`, which is optimized for speed and cost.

## Authentication (API Key)

Red Queen uses an API Key for authentication. This is the simplest way to get started with Gemini.

1.  Generate an API Key in [Google AI Studio](https://aistudio.google.com/).
2.  Add it directly to your `config.yaml` in the `analysis` section:

```yaml
detection:
  analysis:
    provider: "gemini-ai"
    api_key: "YOUR_API_KEY_HERE"
    model_name: "gemini-1.5-flash"
```

## Configuration

In your `config.yaml`, ensure the `analysis` section is correctly populated:

| Field | Type | Description |
|-------|------|-------------|
| `provider` | string | Must be set to `gemini-ai`. |
| `api_key` | string | **Required**. Your Gemini API key from Google AI Studio. |
| `model_name` | string | Gemini model to use (e.g., `gemini-1.5-flash` or `gemini-1.5-pro`). |
| `threshold` | float | Confidence threshold (0.0 to 1.0) for threat detection. |
| `target_objects` | list | List of objects or behaviors Gemini should look for. |
| `max_artifact_size` | int | Max file size in bytes (default: 20MB). |
| `endpoint` | string | Optional. Custom base URL for the API (BaseURL). |

## Memory Management

Red Queen is designed for predictable and efficient memory usage:

- **Inline Artifact Processing**: Artifacts are processed as 'InlineData' within the Gemini API call for maximum performance.
- **Strict Size Bounds**: The `max_artifact_size` setting prevents OOM issues from excessively large files.
- **Concurrency Control**: Global `concurrency` setting limits the number of simultaneous analysis tasks.

## Troubleshooting

- **Error: `api_key required`**: Ensure `api_key` is set in your configuration file.
- **Error: `invalid API key`**: Verify your key in Google AI Studio.
- **Quota Issues**: Check your usage limits in the [Google AI Studio Console](https://aistudio.google.com/app/plan_and_billing).
