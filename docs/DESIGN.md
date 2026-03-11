# Red Queen: System Design Documentation

## System Architecture

The Red Queen system is designed as a modular, event-driven application written in Go. It uses an internal orchestrator to coordinate between ingestion, analysis, storage, and notification components using a unified **Event** model.

```mermaid
graph TD
    subgraph "Ingestion Layer"
        Camera[Networked Camera] -->|FTP Upload| FTPServer[FTP Ingestion Service]
    end

    subgraph "Core Orchestrator"
        FTPServer -->|"1. New Upload (IP)"| ZoneManager[Zone Manager]
        ZoneManager -->|"2a. Unknown IP"| Discard[Discard Task]
        ZoneManager -->|"2b. Known IP (Zone)"| Coordinator[Analysis Coordinator]
        
        Coordinator -->|"3. Create Event (ID, Path, Zone)"| Coordinator
        Coordinator -->|"4. Analyze(Event)"| MLInterface[ML Interface]
        MLInterface -->|"5. Analysis Result"| Coordinator
        
        Coordinator -.->|"5a. Soft Failure"| Retry[Retry with Exponential Backoff]
        Retry -.-> MLInterface

        Coordinator -->|"6. If Threat: Save(Event)"| StorageInterface[Artifact Storage Interface]
        Coordinator -->|"7. If Threat: Send(Event, Result, URL)"| NotificationInterface[Notification Interface]
        
        Coordinator -->|"8. Cleanup"| TempStorage[Ephemeral Storage]
    end

    subgraph "Pluggable Backends"
        MLInterface -.->|Cloud/Local| MLModel[ML Model Provider]
        StorageInterface -.->|S3/LocalFS| StorageProvider[Storage Provider]
        NotificationInterface -.->|Slack/Email| NotificationProvider[Notification Provider]
    end
```

## Core Data Model: The Event

To ensure consistency across all modules, the system uses a structured **Event** object:
- **ID**: A unique UUID for tracking the lifecycle of an upload.
- **FilePath**: The local path to the artifact (image/video).
- **Timestamp**: The precise time the upload was completed.
- **CameraIP**: The source IP address for identification.
- **Zone**: The human-readable zone tag resolved by the Zone Manager.

## System Components

### 1. Ingestion Service (FTP Server)
- **Responsibility**: Provides an FTP endpoint for cameras.
- **Mechanism**: Captures the source IP and file path. Upon completion, it notifies the Zone Manager.

### 2. Zone Manager
- **Responsibility**: Resolves IP addresses to **ZONES**.
- **Role**: Discards unauthorized traffic and enriches authorized uploads with zone context.

### 3. Analysis Coordinator (The "Orchestrator")
- **Responsibility**: Manages the lifecycle of the **Event**.
- **Workflow**:
    - Generates a unique Event ID.
    - Orchestrates ML analysis with retry logic.
    - Coordinates storage and notifications if a threat is confirmed.
    - Ensures the local file is deleted after processing.

### 4. ML Interface (Pluggable)
- **Interface**: `Analyze(ctx, Event) (Result, error)`
- **Error Handling**: Distinguishes between **Soft** (retryable) and **Hard** (fatal) failures.
- **Implementations**:
    - **Gemini AI (Gemini)**: Uses Google's multimodal Gemini models for advanced video understanding. It is configured via `project_id`, `location`, and `model_name`. It uses structured JSON output for reliable parsing. See [Gemini AI Setup Guide](GEMINI_AI.md) for authentication and setup instructions.
    - **Always (Debug)**: A passthrough analyzer that always returns a threat. Useful for testing storage and notification logic without external dependencies.

### 5. Artifact Storage Interface (Pluggable)
- **Interface**: `Save(ctx, Event) (URL, error)`
- **Responsibility**: Persists the artifact and returns a referenceable URL.
- **Implementations**:
    - **Local Storage**: Moves flagged artifacts to a permanent root directory structured by date and zone (`root_path/YYYY-MM-DD/zone/eventID_filename`).

### 6. Notification Interface (Pluggable)
- **Interface**: `Send(ctx, Event, Result, URL) error`
- **Responsibility**: Delivers contextual alerts to external channels.
- **Implementations**:
    - **Webhook Notifier**: Sends a JSON POST request with event details and a link to the stored artifact.
    - **Homey Notifier**: Supports both Homey Cloud and Homey Pro (Local). It triggers flows with a custom tag containing the alert message and artifact URL.

### 7. REST API Server
- **Responsibility**: Serves stored artifacts and provides health/telemetry monitoring.
- **Endpoints**:
    - `/artifacts/{date}/{zone}/{filename}`: Serves recorded threat artifacts from local storage.
    - `/health`: Simple health check endpoint.
    - `/metrics`: Prometheus metrics for system monitoring.

---

## Deployment

### Docker
The system can be deployed using Docker for consistent environments across local and cloud servers.

1. **Build the image**:
   ```bash
   docker build -t red-queen .
   ```

2. **Run with Docker Compose**:
   ```bash
   docker-compose up -d
   ```

3. **Volumes**:
   - `/config`: Mount your `config.yaml` here.
   - `/data/uploads`: Temporary directory for FTP uploads.
   - `/data/storage`: Permanent archive for threat artifacts.

---

## Data Flow Diagram

```mermaid
sequenceDiagram
    participant Camera
    participant FTPServer
    participant ZoneManager
    participant Coordinator
    participant MLInterface
    participant Storage
    participant Notification

    Camera->>FTPServer: "Upload artifact (from IP)"
    FTPServer->>ZoneManager: "New Upload (path, IP)"
    
    alt Known IP
        ZoneManager->>Coordinator: "Validated Upload (path, IP, Zone)"
        Note over Coordinator: Create Event (ID, Timestamp)
        loop Analysis Retry Loop
            Coordinator->>MLInterface: "Analyze(Event)"
            alt Success
                MLInterface-->>Coordinator: "Result (IsThreat: True)"
                
                Coordinator->>Storage: "Save(Event)"
                Storage-->>Coordinator: "Artifact URL"
                
                Coordinator->>Notification: "Send(Event, Result, URL)"
                Notification-->>Coordinator: Success
            else Soft Failure
                MLInterface-->>Coordinator: "Error (Soft)"
                Note over Coordinator: Exponential Backoff
            else Hard Failure
                MLInterface-->>Coordinator: "Error (Hard)"
            end
        end
    else Unknown IP
        ZoneManager-->>FTPServer: Discard & Cleanup
    end
    
    Coordinator->>FTPServer: Final Cleanup
```
