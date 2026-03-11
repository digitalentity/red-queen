# Red Queen: GenAI Development Standards & Style Guide

This document outlines the Go development standards, idiomatic patterns, and best practices for the Red Queen project. All AI assistants should adhere to these guidelines when suggesting or implementing changes.

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
