# Red Queen: Video Surveillance Threat Analysis System

## Overview
Red Queen is a Go-based system designed to analyze video surveillance data for threats, issue real-time alerts, and manage long-term storage of relevant footage. It acts as an intelligent intermediary between networked cameras and notification/storage backends.

## Camera Integration & Ingestion
- **Source**: Integrated with networked cameras capable of motion detection and file upload (images/video clips).
- **Ingestion Protocol**: The system exposes a built-in FTP server with basic authentication (preconfigured credentials).
- **Processing Trigger**: The system monitors the FTP upload process and triggers analysis upon file completion.
- **Ephemeral Storage**: Uploaded files are stored in a temporary location, analyzed, and then discarded unless identified as a threat.

## Video Processing & Threat Detection
- **ML Interface**: A generic, pluggable interface that supports both locally hosted (e.g., sidecar containers, Go bindings) and cloud-based ML models.
- **Analysis Logic**: The system feeds artifacts into the model to identify "suspicious" activity (e.g., unauthorized human presence, specific object detection).
- **Decision Engine**: If the model's confidence score exceeds a configurable threshold, the system flags the artifact for permanent storage and notification.

## Artifact Storage
- **Interface**: A generic, pluggable API for storage providers (e.g., Local File System, AWS S3, Google Cloud Storage).
- **Metadata**: Flagged artifacts should be stored along with relevant metadata (Timestamp, Camera ID, Threat Type, Confidence Score).

## Notification System
- **Interface**: A pluggable notification system to support various channels (e.g., Webhooks, Slack, Email, SMS).
- **Payload**: Notifications must include event details and a reference/link to the stored artifact.

## Operational Requirements
- **Configuration**: System parameters (FTP ports, credentials, ML API keys, storage backends) must be manageable via environment variables or a configuration file.
- **Concurrence**: The system must handle simultaneous uploads from multiple cameras without blocking the analysis pipeline.
- **Logging**: Comprehensive logging for ingestion events, ML results, and system health.
