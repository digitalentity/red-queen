# Red Queen: Video Surveillance Threat Analysis System

## Overview
Red Queen is a Go-based system designed to analyze video surveillance data for threats, issue real-time alerts, and manage long-term storage of relevant footage. It acts as an intelligent intermediary between networked cameras and notification/storage backends.

## Camera Integration & Ingestion
- **Source**: Integrated with networked cameras capable of motion detection and file upload (images/video clips).
- **Registration & Zones**: Each camera must be registered in the system and annotated with a **ZONE** tag (e.g., "North Gate", "Parking Lot").
- **Identification**: Cameras are identified by their **IP address**.
- **Filtering**: The system must discard any uploads from unknown/unregistered IP addresses.
- **Ingestion Protocol**: The system exposes a built-in FTP server with basic authentication (preconfigured credentials).
- **Processing Trigger**: The system monitors the FTP upload process and triggers analysis upon file completion.
- **Ephemeral Storage**: Uploaded files are stored in a temporary location, analyzed, and then discarded unless identified as a threat.

## Video Processing & Threat Detection
- **ML Interface**: A generic, pluggable interface that supports both locally hosted (e.g., sidecar containers, Go bindings) and cloud-based ML models.
- **Analysis Logic**: The system feeds artifacts into the model to identify "suspicious" activity (e.g., unauthorized human presence, specific object detection).
- **Contextual Analysis**: Threat detection can be scoped by **ZONE** (e.g., different sensitivity levels or object types per zone).
- **Error Handling & Resilience**:
    - **Failure Classification**: The system must distinguish between **Hard Failures** (e.g., invalid file format, model-rejected content) and **Soft Failures** (e.g., network timeout, API quota exceeded, temporary service unavailability).
    - **Retry Mechanism**: In the event of a **Soft Failure**, the system must automatically retry the analysis using an **exponential backoff** strategy.
    - **Persistence**: If the maximum number of retries is exceeded for a soft failure, the system should log a critical error and ensure cleanup.
- **Decision Engine**: If the model's confidence score exceeds a configurable threshold, the system flags the artifact for permanent storage and notification.

## Artifact Storage
- **Interface**: A generic, pluggable API for storage providers (e.g., Local File System, AWS S3, Google Cloud Storage).
- **Metadata**: Flagged artifacts should be stored along with relevant metadata (Timestamp, Camera ID/IP, **ZONE**, Threat Type, Confidence Score).

## Notification & Integration
- **Notification Interface**: A pluggable notification system to support various channels (e.g., Webhooks, Slack, Email, SMS).
- **Zone State API (Future)**: An HTTP REST endpoint for receiving/querying notifications about the state of a specific **ZONE**.
- **Payload**: Notifications must include event details, zone information, and a reference/link to the stored artifact.

## Operational Requirements
- **Configuration**: System parameters (FTP ports, camera registration, ML settings, storage backends, and notifications) must be manageable via a **YAML configuration file**.
- **Environment Variables**: Sensitive data (e.g., passwords, API keys) within the YAML should support override or injection via environment variables.
- **Concurrence**: The system must handle simultaneous uploads from multiple cameras without blocking the analysis pipeline.
- **Logging**: Comprehensive logging for ingestion events, ML results, and system health.
