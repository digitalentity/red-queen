# Code Review Findings: Red Queen

This document summarizes the results of a comprehensive code review of the Red Queen surveillance system.

## 1. Concurrency & Race Conditions

### Thread-unsafe Mocks (High)
*   **Files:** `internal/storage/mock.go`, `internal/notify/mock.go`
*   **Issue:** The `MockProvider` and `MockNotifier` append to slices (`SavedEvents`, `SentAlerts`) without synchronization (e.g., `sync.Mutex`).
*   **Impact:** While current tests pass, any concurrent tests or integration tests using the FTP server (which triggers `Process` in a goroutine) will trigger data races, leading to flaky tests or panics.

### Double Processing Risk (Medium)
*   **File:** `internal/ftp/server.go`
*   **Issue:** `ObservedFile.Close` triggers processing via `go f.coordinator.Process(...)`.
*   **Impact:** If the FTP server implementation or a client retry triggers multiple `Close()` calls on the same file handle, the same file will be analyzed and stored multiple times, wasting resources and generating duplicate notifications.

## 2. Architectural & Logic Issues

### FTP "Read-Invisibility" (High)
*   **File:** `internal/ftp/server.go`
*   **Issue:** `ObservedFs.OpenFile` creates files with a UUID prefix (e.g., `UUID-image.jpg`) to ensure uniqueness in the shared temp directory. However, `ObservedFs` does not override `Stat`, `Remove`, or `ReadDir`.
*   **Impact:** After a client successfully uploads a file, any subsequent FTP commands like `SIZE`, `MDTM`, or `DELE` will fail because the client requests the original filename, but the filesystem only contains the UUID-prefixed version.

### Directory Structure Loss (Medium)
*   **File:** `internal/ftp/server.go`
*   **Issue:** `ObservedFs.OpenFile` uses `filepath.Base(name)` when creating the unique filename.
*   **Impact:** If a camera attempts to upload to a subfolder (e.g., `/backyard/clip.mp4`), the directory structure is flattened and lost. All files are stored in the root of the `temp_dir`.

### Hardcoded MIME Type (Low)
*   **File:** `internal/ml/vertex.go`
*   **Issue:** `VertexAnalyzer` hardcodes `MIMEType: "video/mp4"`.
*   **Impact:** Security cameras often use different containers (H.264/H.265 raw streams, MKV). Hardcoding the MIME type may cause the Vertex AI API to reject valid video payloads.

## 3. Resource Management

### Memory OOM Risk (High)
*   **File:** `internal/ml/vertex.go`
*   **Issue:** `VertexAnalyzer.Analyze` loads the entire file (up to 20MB) into memory using `os.ReadFile` before sending it to the API.
*   **Impact:** With a high `concurrency` setting, a burst of concurrent uploads can quickly exhaust system memory, leading to Out-Of-Memory (OOM) kills.

## 4. Go Style & Error Handling

### Context Ignored in Storage (Medium)
*   **File:** `internal/storage/local.go`
*   **Issue:** The `copyFile` helper does not respect the `context.Context`.
*   **Impact:** Long-running file copies (e.g., to a slow mount or network drive) cannot be cancelled during a graceful shutdown, potentially delaying the application exit.

### Fragile Model Response Handling (Medium)
*   **File:** `internal/ml/vertex.go`
*   **Issue:** The analyzer relies on `res.Text()` from the GenAI SDK.
*   **Impact:** If the model response is empty (due to safety filters) or contains multiple candidates, `res.Text()` might behave unexpectedly or panic. It is safer to inspect `res.Candidates` or use `res.FirstCandidate()`.

### Missing FTP TLS (Low)
*   **File:** `internal/ftp/server.go`
*   **Issue:** `ftp.MainDriver.GetTLSConfig` returns `nil, nil`.
*   **Impact:** Camera credentials and video data are transmitted over the network in plain text, making them vulnerable to interception.

---

## Recommendations

1.  **Synchronize Mocks:** Add `sync.Mutex` to mock implementations that track state in slices or maps.
2.  **Fix ObservedFs:** Implement filename mapping or rename files *after* they are successfully closed to maintain visibility for FTP clients.
3.  **Stream Video Data:** Update the ML analyzer to avoid loading entire files into memory; stream the `io.Reader` if supported by the SDK.
4.  **Content-Type Detection:** Use `http.DetectContentType` on the first 512 bytes of the upload instead of hardcoding.
5.  **Context-Aware IO:** Use context-aware I/O (e.g., `io.Copy` with a custom reader that checks `ctx.Done()`) to ensure the system responds correctly to shutdowns.
