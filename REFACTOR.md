# Red Queen: Code Audit & Improvements

This document tracks the results of system audits, identifies areas for improvement, and records completed refactoring tasks.

## 1. Open Issues & Future Refinements

### 1.1. Medium Priority: Memory Pressure in ML Analysis
**Issue:** `VertexAnalyzer.Analyze` currently reads the entire artifact into memory (`os.ReadFile`) before sending it to the Gemini API. 
**Risk:** While protected by `MaxArtifactSize` (default 20MB), this approach can cause memory spikes under high concurrency.
**Proposed Fix:** 
- Implement streaming for the Vertex AI Files API for larger artifacts.
- Explore processing video in segments or using lower-resolution proxies if cloud costs or memory become a bottleneck.

### 1.2. Low Priority: Advanced Camera Protocol Support
**Proposed Fix:** 
- See `FUTURE_WORK.md` for RTSP/RTMP ingestion and ONVIF integration plans.

## 2. Completed Improvements

### 2.1. Concurrency & Lifecycle Management
- **Concurrency Control**: Implemented a semaphore (`chan struct{}`) in the `Coordinator` using the `Concurrency` configuration field to prevent resource exhaustion during bursts of uploads.
- **Structured App Initialization**: Refactored the monolithic `main.go` into a structured `App` component in `internal/app`. This improved testability and established a clear lifecycle (`New`, `Start`, `Stop`).
- **Context & Timeout Management**: Added `ProcessTimeout` to ensure the entire analysis-to-notification pipeline is bound by a deadline (default 5m), preventing hanging goroutines.

### 2.2. Error Handling & Observability
- **Metric Labeling Precision**: Added `Type() string` to `Notifier` and `Provider` interfaces. The `Coordinator` now uses these dynamic values for Prometheus labels, enabling granular monitoring of specific backends.
- **Error Wrapping**: Implemented `Unwrap()` on `ml.AnalysisError`, allowing for idiomatic error inspection using `errors.Is` and `errors.As`.
- **Improved Retry Logic**: Standardized on exponential backoff for soft failures in ML analysis, while respecting context deadlines.

### 2.3. System Integration & Reliability
- **REST API Decoupling**: Decoupled `pkg/api/server.go` from `config.LocalConfig`. It now accepts a generic `http.Handler`, allowing it to support various storage providers (Local, S3, etc.) without code changes.
- **Standardized HTTP Clients**: Implemented a shared, configurable `http.Client` injected into all notifiers, ensuring consistent timeouts and centralized network configuration.
- **Telegram Reliability**: Implemented automatic truncation of captions (1024 chars) and text messages (4096 chars) in `TelegramNotifier` to comply with API limits.
- **MIME Type Detection**: Implemented robust MIME type sniffing and extension fallbacks in `VertexAnalyzer` to support various image and video formats.
- **Secure FTP Naming**: Implemented a unique, traceable naming scheme (`IP-UUID-filename`) for temporary FTP uploads.
