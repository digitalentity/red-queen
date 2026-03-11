# Code Review — Red Queen

## Summary

Overall the codebase is well-structured, with clean interface design, good separation of concerns, and solid test coverage. The findings below are grouped by severity.

---

## Bugs

### B1 — `errors.As` must replace type assertion for wrapped errors
**File:** `internal/coordinator/coordinator.go:165`

```go
// current — breaks if error is wrapped (e.g. by fmt.Errorf("...: %w", aErr))
if aErr, ok := err.(*ml.AnalysisError); ok {
```

A direct type assertion only matches the exact dynamic type. Any caller that wraps the error before returning it will cause this branch to be silently skipped and every soft failure will be retried indefinitely instead of being handled correctly.

**Fix:**
```go
var aErr *ml.AnalysisError
if errors.As(err, &aErr) {
    if aErr.Type == ml.ErrorHard {
        return backoff.Permanent(aErr)
    }
    log.Warn("Soft failure in ML analysis, retrying...", zap.Error(err))
    return err
}
```

---

### B2 — Wrong sentinel error used to detect FTP server shutdown
**File:** `internal/app/app.go:159`

```go
if err := a.ftpServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
    a.logger.Fatal("FTP server failed", zap.Error(err))
}
```

`http.ErrServerClosed` is a sentinel from `net/http` that only `(*http.Server).ListenAndServe` returns. `fclairamb/ftpserverlib` never returns this value, so normal shutdown will always trigger a `Fatal` log.

**Fix:** Check the error returned by the FTP library's own stop flow, or simply accept any non-nil error from `Start()` after `Stop()` has been called by storing a flag, similar to how it's already done for the REST API with `http.ErrServerClosed`.

---

### B3 — `config` package name shadowed by local variable in `gemini.go`
**File:** `internal/ml/gemini.go:112`

```go
config := &genai.GenerateContentConfig{ ... }
```

The local variable `config` shadows the imported package `redqueen/internal/config`. While the code compiles today because `config.MLConfig` is only referenced before this line, any future code referencing the `config` package after line 112 will silently use the local variable instead.

**Fix:** Rename the local variable:
```go
genCfg := &genai.GenerateContentConfig{ ... }
// ...
res, err := a.client.Models.GenerateContent(ctx, a.cfg.ModelName, contents, genCfg)
```

---

### B4 — `truncate` counts bytes, not runes; can produce invalid UTF-8
**File:** `internal/notify/telegram.go:184-188`

```go
func (n *TelegramNotifier) truncate(s string, limit int) string {
    if len(s) <= limit {
        return s
    }
    return s[:limit-3] + "..."
}
```

`len(s)` counts bytes. Slicing a string at a byte offset can split a multi-byte UTF-8 sequence (e.g. emoji, Cyrillic), producing an invalid string and a malformed Telegram request.

**Fix:**
```go
func (n *TelegramNotifier) truncate(s string, limit int) string {
    runes := []rune(s)
    if len(runes) <= limit {
        return s
    }
    return string(runes[:limit-3]) + "..."
}
```

---

### B5 — Partial destination file left behind on context cancellation
**File:** `internal/storage/local.go:51-88`

When `ctx.Done()` fires mid-copy, the function returns `ctx.Err()` but leaves a partially written file at `dst`. Subsequent reads of that artifact will serve corrupt data.

**Fix:** Clean up the destination file on any error path:
```go
func (s *LocalStorage) copyFile(ctx context.Context, src, dst string) (retErr error) {
    // ... open files ...
    defer func() {
        destFile.Close()
        if retErr != nil {
            os.Remove(dst)
        }
    }()
    // ... copy loop ...
}
```

---

## Style & Correctness Issues

### S1 — Error details dropped in `main.go` `Fatal`/`Error` calls
**File:** `cmd/red-queen/main.go:31, 36, 46`

```go
logger.Fatal("Failed to initialize application")  // err silently dropped
logger.Fatal("Failed to start application")        // err silently dropped
logger.Error("Error during shutdown")              // err silently dropped
```

All three calls have the error in scope but don't pass `zap.Error(err)`. The process exits (or logs) with no actionable information.

**Fix:** Add `zap.Error(err)` to each call:
```go
logger.Fatal("Failed to initialize application", zap.Error(err))
```

---

### S2 — `cfg.URL` has dual conflicting roles in `TelegramNotifier`
**File:** `internal/notify/telegram.go:26-29, 67-69`

`cfg.URL` is used both as the Telegram API base URL override (for testing) and as the application's public URL prefix for building artifact links in `formatMessage`. These two semantics conflict: in a non-test environment, a Telegram notifier with a URL override for the API would produce broken artifact links.

**Fix:** Separate the concerns. Add a dedicated `ArtifactBaseURL` field to `NotifyConfig`, or pass the artifact base URL through `Send` rather than embedding it in the notifier config.

---

### S3 — `http.Server` in `pkg/api/server.go` has no timeouts
**File:** `pkg/api/server.go:51-54`

```go
s.server = &http.Server{
    Addr:    fmt.Sprintf(":%d", s.cfg.Port),
    Handler: mux,
}
```

Without `ReadHeaderTimeout`, `ReadTimeout`, and `WriteTimeout`, the server is vulnerable to Slowloris-style connection exhaustion attacks.

**Fix:**
```go
s.server = &http.Server{
    Addr:              fmt.Sprintf(":%d", s.cfg.Port),
    Handler:           mux,
    ReadHeaderTimeout: 10 * time.Second,
    ReadTimeout:       30 * time.Second,
    WriteTimeout:      60 * time.Second,
    IdleTimeout:       120 * time.Second,
}
```

---

### S4 — Old-style build constraint in integration test
**File:** `tests/integration_test.go:1`

```go
// +build integration
```

This is the pre-Go 1.17 syntax. Since the module targets `go 1.24.0`, the modern form is required to guarantee correct behaviour with `go test`:

```go
//go:build integration
```

(Note: no space between `//` and `go:build`.)

---

### S5 — Import grouping order is non-standard in several files
**Files:** `internal/coordinator/coordinator.go`, `internal/notify/telegram.go`, `internal/zone/manager_test.go`

The Go convention enforced by `goimports` is: stdlib → third-party → internal. Several files order imports as stdlib → internal → third-party, which causes `goimports` to reformat them on every save and makes diffs noisier.

**Example fix for `coordinator.go`:**
```go
import (
    "context"
    "os"
    "time"

    "github.com/cenkalti/backoff/v4"
    "github.com/google/uuid"
    "go.uber.org/zap"

    "redqueen/internal/metrics"
    "redqueen/internal/ml"
    "redqueen/internal/models"
    "redqueen/internal/notify"
    "redqueen/internal/storage"
)
```

---

### S6 — `fakeFileInfo.Sys()` returns `interface{}` instead of `any`
**File:** `internal/ftp/server.go:377`

```go
func (f *fakeFileInfo) Sys() interface{} { return nil }
```

Since Go 1.18 `any` is the preferred alias. More importantly, the `os.FileInfo` interface as of Go 1.23 declares `Sys() any`. Using `interface{}` works due to type identity, but is inconsistent with the rest of the codebase.

**Fix:**
```go
func (f *fakeFileInfo) Sys() any { return nil }
```

---

### S7 — `getRegistry` test helper defined in production-adjacent file
**File:** `internal/ftp/server_test.go:114-123`

```go
func (d *MainDriver) getRegistry(ip string) *VirtualRegistry {
    // ...
}
```

A method is added to `MainDriver` inside a `_test.go` file. This is valid Go (test-only method), but it exposes registry access through a method that duplicates what `AuthUser` already does inline. Consider either making the method part of the real implementation (useful) or restructuring the test to use `AuthUser` directly.

---

### S8 — Spurious double blank line in `config.go`
**File:** `internal/config/config.go:23-24`

There is an extra blank line between `HTTPClientConfig` and `FTPConfig`. `gofmt` only allows one blank line between declarations.

---

### S9 — Stale `Channel` field in `NotifyConfig`
**File:** `internal/config/config.go:76`

```go
Channel string `mapstructure:"channel"` // Used by Slack
```

Slack support is not implemented anywhere in the codebase. The field is dead configuration that will confuse users reading the config reference. Remove it or gate it behind a `// TODO` with a tracking comment.

---

## Design Observations

### D1 — `ProcessTimeout` and `HTTPClient.Timeout` parsed at runtime, not load time
**Files:** `internal/app/app.go:39-45, 124-130`

Both duration strings are silently defaulted if malformed. This is acceptable, but moving the parsing into `LoadConfig` and storing `time.Duration` fields directly would catch misconfiguration at startup instead of silently degrading.

---

### D2 — `MockManager` and `MockProvider` compiled into production binary
**Files:** `internal/zone/mock.go`, `internal/storage/mock.go`, `internal/notify/mock.go`, `internal/ml/mock.go`

These mocks live in non-test files and are compiled into every build. The standard Go pattern is to place them in `*_test.go` files or a dedicated `internal/*/testing` subpackage. As-is, `storage.MockProvider` is also used in production fallback paths in `app.go` (lines 83, 89), which is a stronger reason to keep it out of test-only files — but the `zone`, `notify`, and `ml` mocks are only used in tests and should be moved.

---

### D3 — `coordinator.Coordinator` is a concrete type, not an interface
**File:** `internal/ftp/server.go:88, internal/app/app.go:29`

`coordinator.Coordinator` is referenced directly by pointer everywhere. This makes it impossible to inject a mock coordinator in tests without constructing a real one (see `server_test.go:91` zero-value workaround). Introducing a `coordinator.Processor` interface (with a single `Process` method) would decouple the FTP server from the coordinator implementation.

---

### D4 — Integration test uses `time.Sleep` for startup synchronisation
**File:** `tests/integration_test.go:128`

```go
time.Sleep(2 * time.Second)
```

Fixed sleeps are a common source of test flakiness. Replacing this with a retry loop that polls `/health` until it responds (or times out) would make the test both faster and more reliable.
