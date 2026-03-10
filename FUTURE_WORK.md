# Future Work: Red Queen Improvements

This document outlines planned and suggested improvements for the Red Queen Video Surveillance Threat Analysis system.

## 1. Ingestion & Camera Integration
- **Advanced Protocols**: Add support for RTSP and RTMP ingestion to support real-time streaming analysis rather than just file-based FTP uploads.
- **ONVIF Support**: Implement ONVIF integration to allow the system to control PTZ (Pan-Tilt-Zoom) cameras, such as automatically centering on a detected threat.
- **Motion Pre-filtering**: Implement a local, lightweight motion detection filter (e.g., using OpenCV) to avoid sending static frames to expensive Cloud ML models.

## 2. Machine Learning & Analysis
- **Edge ML Provider**: Implement a local provider using TensorFlow Lite or ONNX to perform analysis on-premise, reducing latency and cloud costs.
- **Face Recognition**: Integrate a face recognition module to distinguish between "Authorized Personnel" and "Intruders."
- **Behavioral Analysis**: Move beyond object detection to analyze behavior (e.g., a person loitering or climbing a fence).

## 3. Storage & Data Management
- **Cloud Storage Providers**: Implement the S3 and Google Cloud Storage (GCS) providers defined in the configuration.
- **Metadata Database**: Integrate a database (SQLite or PostgreSQL) to store event metadata. Currently, the system relies on the filesystem, making it difficult to search or filter historical events.
- **Retention Policies**: Implement an automated cleanup service to delete artifacts and metadata older than a configurable number of days.

## 4. API & User Interface
- **Web Dashboard**: Build a modern web interface (React/Vue) to allow security personnel to view live alerts, browse historical artifacts, and manage camera zones.
- **API Security**: Add JWT or API Key authentication to the REST API server to secure access to threat artifacts.
- **Zone State API**: Complete the REST endpoints to allow external systems to query the current "Security State" of a specific zone.

## 5. Observability & Reliability
- **Prometheus Metrics**: Export system metrics (e.g., number of uploads, ML latency, storage usage, notification success rates).
- **OpenTelemetry Tracing**: Implement tracing to follow the lifecycle of an event from the moment a file is uploaded until the notification is sent.
- **Health Checks**: Enhance the `/health` endpoint to report the status of downstream dependencies (Vertex AI, Storage, etc.).

## 6. Notification Channels
- **Mobile Push**: Native mobile app integration for real-time push notifications.
- **Matrix/Element**: Support for decentralized, secure messaging via the Matrix protocol.
- **SMS/Voice**: Integration with Twilio for critical alerts that require immediate attention.
