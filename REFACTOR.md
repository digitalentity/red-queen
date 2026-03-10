# Red Queen: Refactoring & Improvements

Based on a comprehensive review of the codebase, the following issues have been identified. This document outlines proposed fixes to improve system stability, performance, and maintainability.

## 1. High Priority: Memory Management in ML Analysis (Resolved)
**Issue:** `internal/ml/vertex.go` previously loaded files without limits, risking OOM crashes for large artifacts.
**Fix:** 
- Added `MaxArtifactSize` to `MLConfig` to enforce a hard limit on artifact size before processing.
- Implemented file size check and MIME type detection in `VertexAnalyzer`.
- Using `InlineData` for low-latency analysis of small artifacts as recommended by Gemini API for current use cases.
- **Future Note:** If larger videos (>20MB) are required, the Files API (streaming) should be re-implemented.

## 2. High Priority: Lack of Concurrency Control
**Issue:** The FTP server triggers `Coordinator.Process` in a new goroutine for every file upload without any limits.
**Impact:** System resources (CPU, Memory, Network) can be exhausted by a burst of uploads. Vertex AI API quotas may also be quickly exceeded.
**Proposed Fix:** 
- Utilize the `Concurrency` field in `Config` (currently unused).
- Implement a worker pool or use a semaphore (e.g., `chan struct{}`) in the `Coordinator` to limit the number of concurrent analysis tasks.

## 3. High Priority: Insecure/Unreliable FTP Temp Naming (Resolved)
**Issue:** `internal/ftp/server.go` previously generated unique names for uploaded files using `os.Getpid() + original_name`.
**Fix:** 
- Implemented a robust temporary naming scheme: `IP-UUID-original_name.ext`.
- This ensures global uniqueness, easy traceability of the source camera, and preservation of the original file context while aiding in MIME type detection.


## 4. Medium Priority: Metric Labeling Imprecision
**Issue:** `internal/metrics/metrics.go` defines labels like `provider` and `status`, but the `Coordinator` uses hardcoded strings like `"notifier"` or `"local"` instead of the actual implementation type.
**Impact:** Metrics in Prometheus/Grafana will be aggregated incorrectly, making it impossible to distinguish between different notifiers (e.g., Telegram vs. Webhook failure rates).
**Proposed Fix:** 
- Add a `Name()` or `Type()` method to `Notifier`, `Analyzer`, and `Storage` interfaces.
- Use these dynamic values when recording metrics in the `Coordinator`.

## 5. Medium Priority: Context & Timeout Management
**Issue:** `Coordinator.Process` is invoked with `context.Background()`.
**Impact:** Analysis or notification tasks could hang indefinitely if the external service (Vertex AI, Telegram) doesn't respond, blocking system resources.
**Proposed Fix:** 
- Wrap the context with a timeout (e.g., 5 minutes) in `Coordinator.Process`.
- Ensure all downstream calls (ML, Storage, Notifiers) respect this context.

## 6. Medium Priority: Error Classification in Coordinator
**Issue:** In `analyzeWithRetry`, unknown errors default to `backoff.Permanent(err)`.
**Impact:** Transient network errors that are not explicitly wrapped as `ErrorSoft` will cause the analysis to fail immediately instead of retrying.
**Proposed Fix:** 
- Default to `ErrorSoft` for unknown errors, or implement a check for common retryable network errors (e.g., `net.Error.Temporary()`).

## 7. Medium Priority: MIME Type Handling
**Issue:** `VertexAnalyzer` hardcodes `MIMEType: "video/mp4"`.
**Impact:** Analysis will fail or be inaccurate if a camera sends JPEG images or different video formats (e.g., `.mov`, `.mkv`).
**Proposed Fix:** 
- Use `http.DetectContentType` or check file extensions to determine the correct MIME type before calling the ML provider.

## 8. Medium Priority: Monolithic `main.go`
**Issue:** `cmd/red-queen/main.go` contains all initialization logic.
**Impact:** Difficult to unit test the system setup and main lifecycle.
**Proposed Fix:** 
- Refactor the logic into a `System` or `App` struct with `New()`, `Start()`, and `Stop()` methods.

## 9. Low Priority: Standardize HTTP Clients
**Issue:** Multiple notifiers create their own `http.Client` with hardcoded timeouts.
**Impact:** Inconsistent behavior and difficult to tune global timeouts or proxy settings.
**Proposed Fix:** 
- Create a shared, configurable HTTP client or use a factory pattern to provide pre-configured clients to notifiers.
