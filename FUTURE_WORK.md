# Future Work: Red Queen Improvements

This document outlines planned and suggested improvements for the Red Queen Video Surveillance Threat Analysis system.

## 1. Ingestion & Camera Integration
- **Advanced Protocols**: Add support for RTSP and RTMP ingestion to support real-time streaming analysis rather than just file-based FTP uploads.
- **ONVIF Support**: Implement ONVIF integration to allow the system to control PTZ (Pan-Tilt-Zoom) cameras, such as automatically centering on a detected threat.
- **Motion Pre-filtering**: Implement a local, lightweight motion detection filter (e.g., using OpenCV) to avoid sending static frames to expensive Cloud ML models. (The two-stage pipeline infrastructure — `ChainedAnalyzer` and prefilter config — is already in place; what remains is the actual YOLO-ONNX provider, tracked in §2 below.)

## 2. Machine Learning & Analysis
- **ML Pre-classifier / YOLO-ONNX Provider (Cost Optimization)**: The `ChainedAnalyzer` pipeline and prefilter config are implemented; the `yolo-onnx` provider factory stub exists but returns "not yet implemented". Complete the provider using ONNX Runtime Go bindings against a YOLOv11-Nano or MobileNetV4 model. Target ~99% recall to avoid missing suspects while filtering out static noise. A TensorFlow Lite sidecar is an alternative for CPU-only environments.
- **Edge ML Provider**: Implement a local provider using TensorFlow Lite or ONNX to perform analysis on-premise, reducing latency and cloud costs.
- **Face Recognition**: Integrate a face recognition module to distinguish between "Authorized Personnel" and "Intruders."
- **Behavioral Analysis**: Move beyond object detection to analyze behavior (e.g., a person loitering or climbing a fence).

## 3. Storage & Data Management
- **Cloud Storage Providers**: Add Google Cloud Storage (GCS) and S3-compatible providers. Google Drive is already supported; GCS/S3 are not yet implemented.
- **Artifact redirect for remote providers**: The `/artifacts/...` REST endpoint currently returns 404 when no local storage provider is configured. For remote providers (e.g. Google Drive), the handler could issue a `302 Found` redirect to the provider's own URL (e.g. Drive's `webViewLink`), keeping the endpoint a stable, provider-agnostic entry point. Possible design: `storage.Provider` gains an optional `ArtifactURL(id string) (string, bool)` method; the handler checks for a redirect before falling back to local file serving.
- **Metadata Database**: Integrate a database (SQLite or PostgreSQL) to store event metadata. Currently, the system relies on the filesystem, making it difficult to search or filter historical events.
- **Retention Policies**: Implement an automated cleanup service to delete artifacts and metadata older than a configurable number of days.

## 4. API & User Interface
- **Web Dashboard**: Build a modern web interface (React/Vue) to allow security personnel to view live alerts, browse historical artifacts, and manage camera zones.
- **API Security**: Add JWT or API Key authentication to the REST API server to secure access to threat artifacts.
- **Zone State API**: Complete the REST endpoints to allow external systems to query the current "Security State" of a specific zone.

## 5. Observability & Reliability
- **OpenTelemetry Tracing**: Implement tracing to follow the lifecycle of an event from the moment a file is uploaded until the notification is sent.
- **Enhanced Health Checks**: Extend the existing `/health` endpoint to report the status of downstream dependencies (Gemini AI, Storage, etc.). Currently, it only provides a basic "OK" response.

## 6. Notification Channels
- **Mobile Push**: Native mobile app integration for real-time push notifications.
- **Matrix/Element**: Support for decentralized, secure messaging via the Matrix protocol.
- **SMS/Voice**: Integration with Twilio for critical alerts that require immediate attention.
