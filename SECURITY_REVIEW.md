# Security Review: Red Queen

This document summarizes the security review findings for the Red Queen surveillance threat analysis system.

## Summary Table

| Finding ID | Title | Severity | Status |
|------------|-------|----------|--------|
| RQ-SEC-01  | No TLS Support for FTP Ingestion | Critical | Open |
| RQ-SEC-02  | Public REST API and Artifact Access | High | Open |
| RQ-SEC-03  | Potential Disk Exhaustion (DoS) | High | Open |
| RQ-SEC-04  | Shared FTP Credentials for All Cameras | Medium | Open |
| RQ-SEC-05  | Potential Memory Exhaustion (OOM) | Medium | Open |
| RQ-SEC-06  | Missing Environment Variable Binding | Medium | Open |
| RQ-SEC-07  | No Authorization on Webhook Notifications | Medium | Open |
| RQ-SEC-08  | Sensitive Information Logged at Startup | Low | Open |

---

## Detailed Findings

### [RQ-SEC-01] No TLS Support for FTP Ingestion
**Severity:** Critical  
**Location:** `internal/ftp/server.go`  
**Description:** The FTP server implementation explicitly returns an error when TLS configuration is requested (`GetTLSConfig` returns "TLS not supported"). As a result, camera credentials and video/image artifacts are transmitted over the network in cleartext.  
**Impact:** An attacker on the same network can sniff FTP credentials and the media artifacts being uploaded.  
**Recommendation:** Implement FTPS support by providing a valid TLS configuration in `MainDriver.GetTLSConfig`.

### [RQ-SEC-02] Public REST API and Artifact Access
**Severity:** High  
**Location:** `pkg/api/server.go`, `internal/app/app.go`  
**Description:** The REST API does not implement any authentication or authorization mechanism. The `/artifacts/` endpoint serves all stored files using a standard `http.FileServer`.  
**Impact:** Anyone with network access to the API port can download all captured surveillance artifacts and view system metrics (which may contain sensitive IP/Zone info).  
**Recommendation:** Implement API Key or Bearer Token authentication for the REST API. Ensure that the `artifactHandler` only serves files to authenticated users.

### [RQ-SEC-03] Potential Disk Exhaustion (DoS)
**Severity:** High  
**Location:** `internal/coordinator/coordinator.go`  
**Description:** The coordinator triggers a new `Process` goroutine for every file upload. While a semaphore limits concurrent ML analysis, the files are only deleted *after* they are processed or if the context is cancelled.  
**Impact:** An attacker can flood the FTP server with many small files. These files will accumulate in `temp_dir` while waiting for the semaphore, potentially filling the disk and causing a system-wide failure.  
**Recommendation:** Implement disk usage quotas or a hard limit on the number of pending files in the queue.

### [RQ-SEC-04] Shared FTP Credentials for All Cameras
**Severity:** Medium  
**Location:** `internal/ftp/server.go`  
**Description:** The FTP server uses a single username and password for all camera connections.  
**Impact:** Compromise of one camera or its configuration reveals the credentials for all cameras in the system.  
**Recommendation:** Allow per-zone or per-camera FTP credentials in the configuration.

### [RQ-SEC-05] Potential Memory Exhaustion (OOM)
**Severity:** Medium  
**Location:** `internal/ml/gemini.go`  
**Description:** The `GeminiAnalyzer` reads the entire artifact into memory (`os.ReadFile`) to send it as `InlineData` to the Gemini API. While there is a `MaxArtifactSize` check, multiple concurrent analyses (controlled by `Concurrency`) could lead to memory exhaustion.  
**Impact:** High concurrency settings combined with large (20MB+) files can lead to an Out Of Memory crash.  
**Recommendation:** Ensure `Concurrency * MaxArtifactSize` is well within the system's available memory. Consider streaming data to the ML provider if the API supports it.

### [RQ-SEC-06] Missing Environment Variable Binding
**Severity:** Medium  
**Location:** `internal/config/config.go`  
**Description:** `LoadConfig` uses `viper` but does not call `AutomaticEnv()` or bind specific keys to environment variables.  
**Impact:** Users may be forced to hardcode sensitive secrets (API keys, bot tokens, FTP passwords) in the `config.yaml` file, which is often committed to source control or shared.  
**Recommendation:** Call `v.AutomaticEnv()` and use `v.SetEnvKeyReplacer` to allow environment variable overrides for all config fields.

### [RQ-SEC-07] No Authorization on Webhook Notifications
**Severity:** Medium  
**Location:** `internal/notify/webhook.go`  
**Description:** The webhook notifier sends a POST request with the payload but does not support any authentication headers (like `Authorization` or `X-Api-Key`).  
**Impact:** The receiving endpoint cannot verify that the notification originated from the Red Queen system.  
**Recommendation:** Add support for custom headers or a shared secret in the `NotifyConfig` for webhooks.

### [RQ-SEC-08] Sensitive Information Logged at Startup
**Severity:** Low  
**Location:** `internal/app/app.go`  
**Description:** The application logs webhook URLs, Telegram chat IDs, and Homey IDs during initialization.  
**Impact:** While not directly exploitable, this information is leaked to the log stream, which might be stored in insecure locations.  
**Recommendation:** Redact sensitive parts of URLs or tokens in the logs.
