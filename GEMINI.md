# Red Queen: Project Instructions & Development Guide

## Overview
Red Queen is a modular, event-driven video surveillance threat analysis system written in Go. It ingests video/image uploads via FTP, analyzes them using pluggable ML providers (like Gemini AI), and triggers notifications (Webhook, Homey, Telegram) if a threat is detected.

## Project Structure
- `cmd/red-queen/`: Main entry point and system initialization.
- `internal/`: Core business logic.
    - `config/`: Configuration loading and logging.
    - `coordinator/`: The orchestrator that manages the event lifecycle.
    - `ftp/`: FTP server for camera ingestion.
    - `ml/`: Pluggable ML analysis interfaces and implementations.
    - `models/`: Common data models (Event, Result).
    - `notify/`: Pluggable notification providers (Telegram, Webhook, Homey).
    - `storage/`: Pluggable artifact storage providers.
    - `zone/`: IP-to-Zone resolution and management.
- `pkg/api/`: REST API for serving artifacts and metrics.
- `docs/`: Design documentation and feature guides.
- `tests/`: Integration tests.

## Prerequisites
- **Go**: 1.24.0 or later.
- **Docker**: For containerized deployment.
- **Make**: For running common tasks.

## Development Workflow
Follow the **Research -> Strategy -> Execution** lifecycle for all changes.

### 1. Research & Strategy
- Map the codebase using `grep_search` and `glob`.
- Review `docs/DESIGN.md` for architectural context.
- For new features, create a design document in `docs/` before implementation.

### 2. Execution
- **Surgical Changes**: Keep changes focused and idiomatic.
- **Dependencies**: Use standard library where possible (e.g., `net/http` for API clients).
- **Configuration**: Update `internal/config/config.go` if new parameters are needed.
- **Formatting**: Always run `make build` to ensure the code compiles and follows Go standards.

### 3. Validation (Mandatory)
- **Unit Tests**: Add tests in the same directory as the implementation (e.g., `internal/notify/telegram_test.go`).
- **Integration Tests**: Add end-to-end tests in `tests/integration_test.go`.
- **Run all tests**:
  ```bash
  make test
  make integration-test
  ```

### 4. Commit Guidelines
- Always include a `Co-Authored-By: Gemini CLI <gemini-code-assist@google.com>` sign-off at the end of the commit message.
- Use the imperative mood in the subject line (e.g., "Add feature" instead of "Added feature").
- Provide a clear and concise description of the changes in the commit body.

## Common Tasks (Makefile)
- `make build`: Compiles the `red-queen` binary.
- `make test`: Runs all unit tests.
- `make integration-test`: Runs integration tests (requires a build).
- `make run`: Builds and starts the system.
- `make clean`: Removes the compiled binary.

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

### Configuration (`internal/config/config.go`)

Loaded by viper. Duration fields (`ProcessTimeout`, `HTTPClient.Timeout`) are `time.Duration` — viper decodes YAML strings like `"5m"` automatically. `NotifyConfig` is a flat struct shared by all notifier types.

### Metrics

Prometheus metrics are registered via `promauto` in `internal/metrics/metrics.go` and exposed at `/metrics` by the API server. Labels follow the pattern `zone` / `provider` / `status`.

### Testing conventions

- Mocks (`MockAnalyzer`, `MockProvider`, `MockNotifier`) live in non-test files because they are used as production no-op fallbacks in `app.go`. `MockManager` (zone) lives in `zone/mock_test.go` because it is test-only.
- Integration test (`tests/integration_test.go`, build tag `//go:build integration`) compiles and runs the real binary, uploads via `curl`, and asserts on webhook/Telegram payloads and file presence. It requires a pre-built binary (`make integration-test` handles this).

## Adding a New Notifier
1. Define any new configuration fields in `internal/config/config.go`.
2. Implement the `Notifier` interface in `internal/notify/`.
3. Register the new notifier in the initialization loop within `internal/app/app.go` (specifically in `RegisterNotifiers`).
4. Add unit tests in `internal/notify/your_notifier_test.go`.
5. Document the new notifier in `docs/`.

## Development Standards & Style Guide
For a comprehensive guide on Go development standards, idiomatic patterns, concurrency, error handling, and more, refer to [docs/GENAI.md](docs/GENAI.md). All code contributions should adhere to these guidelines.
