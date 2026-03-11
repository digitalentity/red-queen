# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build              # Build binary → ./red-queen
make test               # Run all unit tests
make integration-test   # Build binary then run integration tests (requires compiled binary)
make run                # Build and run with config.yaml in CWD
make clean              # Remove compiled binary

# Single package test
go test ./internal/coordinator/...

# Single test function
go test ./internal/coordinator/ -run TestCoordinator_Process_Threat

# Integration tests require a built binary and the mock-threat flag
RED_QUEEN_MOCK_THREAT=true ./red-queen
```

Configuration is loaded from the path in `RED_QUEEN_CONFIG` env var, or `./config.yaml` by default. Copy `config.example.yaml` to get started. See `docs/DESIGN.md` for broader architectural context and `docs/GEMINI_AI.md` for Gemini setup.

## Development Workflow

Follow the **Research -> Strategy -> Execution** lifecycle for all changes:
1. **Research**: Map the codebase and validate assumptions (use `grep` and `find`).
2. **Strategy**: Formulate a grounded plan and share a concise summary.
3. **Execution**: Apply surgical changes and validate with tests.

## Go Standards & Best Practices

- **Idiomatic Go**: Follow Effective Go; use `gofmt` and `goimports`.
- **Error Handling**: Wrap errors with context; use `errors.As` for typed errors.
- **Concurrency**: Use context for cancellation/deadlines; manage goroutine lifecycles.
- **Testing**: Use table-driven tests with subtests; use interfaces for dependency injection.
- **Performance**: Benchmark critical paths; avoid unnecessary allocations.
- **Documentation**: Document all exported items using standard Go documentation comments.

Refer to [docs/GENAI.md](docs/GENAI.md) for the full comprehensive development checklist and idiomatic patterns.

## Architecture

The system is a pipeline: **FTP upload → Zone lookup → Coordinator → ML analysis → Storage + Notifications**.

### Event lifecycle (`internal/coordinator/coordinator.go`)

`Coordinator.Process` is the central method. It runs synchronously (bounded by a semaphore for concurrency) and orchestrates:
1. ML analysis with exponential-backoff retry via `cenkalti/backoff`
2. Artifact storage (only on confirmed threat)
3. Fan-out notification to all configured notifiers
4. Ephemeral file cleanup via a deferred `os.Remove`

Errors from the ML layer are typed (`ErrorHard` skips retries, `ErrorSoft` retries). Always use `errors.As` when unwrapping `*ml.AnalysisError`.

### FTP ingestion (`internal/ftp/server.go`)

Uses `fclairamb/ftpserverlib`. The key design is a **virtual filesystem** (`ObservedFs`) that wraps `afero.Fs`:
- Files uploaded by cameras are stored flat in `TempDir` with UUID-prefixed names to avoid collisions across cameras.
- A `VirtualRegistry` per camera IP maps the virtual path the camera sees (e.g. `/subdir/clip.mp4`) to the physical UUID filename.
- `ObservedFile.Close()` fires `coordinator.Process` in a goroutine using `sync.Once`.
- On `Stop()`, `ftpserverlib` returns `nil` (not `http.ErrServerClosed`); do not use the HTTP sentinel here.

### Interfaces and extension points

All major subsystems are behind interfaces defined in their own packages:

| Interface | Package | Implementations |
|-----------|---------|-----------------|
| `ml.Analyzer` | `internal/ml` | `GeminiAnalyzer`, `PassThroughAnalyzer`, `MockAnalyzer` |
| `storage.Provider` | `internal/storage` | `LocalStorage`, `MockProvider` |
| `notify.Notifier` | `internal/notify` | `WebhookNotifier`, `TelegramNotifier`, `HomeyNotifier`, `MockNotifier` |
| `zone.Manager` | `internal/zone` | `managerImpl` (unexported), `MockManager` (test-only) |
| `coordinator.Processor` | `internal/coordinator` | `*Coordinator` |

To add a new notifier: implement `notify.Notifier`, register it in `RegisterNotifiers` within `internal/app/app.go`, and add config fields to `NotifyConfig` in `internal/config/config.go`.

### Configuration (`internal/config/config.go`)

Loaded by viper. Duration fields (`ProcessTimeout`, `HTTPClient.Timeout`) are `time.Duration` — viper decodes YAML strings like `"5m"` automatically. `NotifyConfig` is a flat struct shared by all notifier types; fields that only apply to one notifier are documented with comments.

### Metrics

Prometheus metrics are registered via `promauto` in `internal/metrics/metrics.go` and exposed at `/metrics` by the API server. Labels follow the pattern `zone` / `provider` / `status`.

### Testing conventions

- Mocks (`MockAnalyzer`, `MockProvider`, `MockNotifier`) live in non-test files because they are used as production no-op fallbacks in `app.go`. `MockManager` (zone) lives in `zone/mock_test.go` because it is test-only.
- Integration test (`tests/integration_test.go`, build tag `//go:build integration`) compiles and runs the real binary, uploads via `curl`, and asserts on webhook/Telegram payloads and file presence. It requires a pre-built binary (`make integration-test` handles this).
