# Setting up Gemini AI (Gemini) for Red Queen

Red Queen uses Google's **Gemini AI (Gemini)** for multimodal video analysis. This requires a Google Cloud project with the Gemini AI API enabled and proper authentication configured.

## Prerequisites

1.  **Google Cloud Project**: Create or use an existing project on [Google Cloud Console](https://console.cloud.google.com/).
2.  **Enable Gemini AI API**: Go to the **Gemini AI** section in the console and enable the API for your project.
3.  **Model Selection**: By default, Red Queen is configured to use `gemini-1.5-flash`, which is optimized for speed and cost.

## Authentication (Application Default Credentials)

The Gemini AI SDK uses **Application Default Credentials (ADC)** to authenticate. If you see an error like `could not find default credentials`, follow these steps:

### Option 1: Local Development (User Credentials)
If you are running the system locally for testing, the easiest way is to use your personal Google account credentials:
1.  Install the [Google Cloud CLI](https://cloud.google.com/sdk/docs/install).
2.  Run the following command:
    ```bash
    gcloud auth application-default login
    ```
    This will open a browser to authenticate and store a JSON file in a well-known location that the SDK will automatically find.

### Option 2: Server/Production (Service Account)
For a permanent server deployment (including Docker), use a **Service Account**:
1.  Go to **IAM & Admin > Service Accounts** in the Google Cloud Console.
2.  Create a service account (e.g., `red-queen-analyzer`).
3.  Grant the account the **Gemini AI User** role (`roles/aiplatform.user`).
4.  Create and download a **JSON Key** for this service account.
5.  Set the `GOOGLE_APPLICATION_CREDENTIALS` environment variable to the path of this JSON file:
    ```bash
    export GOOGLE_APPLICATION_CREDENTIALS="/path/to/your/service-account-key.json"
    ```

### Option 3: Docker Deployment
If running inside Docker, you must mount the service account key and set the environment variable in your `docker-compose.yaml` or `docker run` command:

```yaml
services:
  red-queen:
    image: red-queen:latest
    environment:
      - GOOGLE_APPLICATION_CREDENTIALS=/config/gcp-key.json
      - RED_QUEEN_ML_PROJECT_ID=your-project-id
    volumes:
      - ./secrets/gcp-key.json:/config/gcp-key.json:ro
      - ./config.yaml:/config/config.yaml:ro
```

## Configuration

In your `config.yaml`, ensure the `ml` section is correctly populated:

```yaml
ml:
  provider: "gemini-ai"
  model_name: "gemini-1.5-flash" # or "gemini-1.5-pro"
  project_id: "your-project-id"
  location: "us-central1"
  threshold: 0.85
  target_objects: ["person", "weapon"]
```

## Memory Management

Red Queen is designed for predictable and efficient memory usage when performing ML analysis:

- **Inline Artifact Processing**: For performance, artifacts are processed as 'InlineData' within the Gemini API call. This is the fastest method for the file sizes typical of security cameras.
- **Strict Size Bounds**: The `max_artifact_size` configuration (default 20MB) prevents the system from attempting to analyze excessively large files that could cause memory pressure.
- **Concurrency Control**: Total system memory usage is governed by the `concurrency` setting in the root configuration. This limits the number of simultaneous analysis tasks, ensuring the system remains stable even under high upload volume.

## Troubleshooting

- **Error: `permission denied`**: Ensure the service account has the `Gemini AI User` role.
- **Error: `api not enabled`**: Verify the Gemini AI API is enabled in the Google Cloud Console.
- **Quota Issues**: Check the **Quotas & System Limits** in the Google Cloud Console if you experience frequent `429 Too Many Requests` (Soft Failures).
