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

## Common Tasks (Makefile)
- `make build`: Compiles the `red-queen` binary.
- `make test`: Runs all unit tests.
- `make integration-test`: Runs integration tests (requires a build).
- `make run`: Builds and starts the system.
- `make clean`: Removes the compiled binary.

## Adding a New Notifier
1. Define any new configuration fields in `internal/config/config.go`.
2. Implement the `Notifier` interface in `internal/notify/`.
3. Register the new notifier in the initialization loop within `cmd/red-queen/main.go`.
4. Add unit tests in `internal/notify/your_notifier_test.go`.
5. Document the new notifier in `docs/`.

## Go development checklist:
- Idiomatic code following effective Go guidelines
- gofmt compliance
- Context propagation in all APIs
- Comprehensive error handling with wrapping
- Table-driven tests with subtests
- Benchmark critical code paths
- Race condition free code
- Documentation for all exported items

## Idiomatic Go patterns:
- Interface composition over inheritance
- Accept interfaces, return structs
- Channels for orchestration, mutexes for state
- Error values over exceptions
- Explicit over implicit behavior
- Small, focused interfaces
- Dependency injection via interfaces
- Configuration through functional options

## Concurrency mastery:
- Goroutine lifecycle management
- Channel patterns and pipelines
- Context for cancellation and deadlines
- Select statements for multiplexing
- Worker pools with bounded concurrency
- Fan-in/fan-out patterns
- Rate limiting and backpressure
- Synchronization with sync primitives

## Error handling excellence:
- Wrapped errors with context
- Custom error types with behavior
- Sentinel errors for known conditions
- Error handling at appropriate levels
- Structured error messages
- Error recovery strategies
- Panic only for programming errors
- Graceful degradation patterns

## Performance optimization:
- CPU and memory profiling with pprof
- Benchmark-driven development
- Zero-allocation techniques
- Object pooling with sync.Pool
- Efficient string building
- Slice pre-allocation
- Compiler optimization understanding
- Cache-friendly data structures

## Testing methodology:
- Table-driven test patterns
- Subtest organization
- Test fixtures and golden files
- Interface mocking strategies
- Integration test setup
- Benchmark comparisons
- Fuzzing for edge cases

## Microservices patterns:
- REST API with middleware
- Health checks and readiness
- Graceful shutdown handling
- Configuration management

## Memory management:
- Understanding escape analysis
- Stack vs heap allocation
- Garbage collection tuning
- Memory leak prevention
- Efficient buffer usage
- String interning techniques
- Slice capacity management
- Map pre-sizing strategies

## Build and tooling:
- Module management best practices
- Build tags and constraints
- Cross-compilation setup
- CGO usage guidelines
- Go generate workflows
- Makefile conventions
- Docker multi-stage builds
- CI/CD optimization
