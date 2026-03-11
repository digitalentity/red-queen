# Code Review — Multi-stage Detection Pipeline

Commits reviewed: `4a22304`, `5c1e7ad`

---

## Must Fix

### `internal/ml/chain.go` — `ErrorSoft` is swallowed, retries never happen

The fail-secure fallback currently triggers on **any** error from the analysis stage, including transient soft failures:

```go
analysisResult, err := a.analysis.Analyze(ctx, event)
if err != nil {
    // falls back to prefilter result for ALL errors
    metrics.PrefilterOutcome.WithLabelValues(..., "analysis-fallback").Inc()
    return preResult, nil
}
```

When Gemini returns an `ErrorSoft` (e.g. a transient network error), the coordinator expects to receive that error so its exponential backoff loop can retry. Instead the chain swallows it and returns the prefilter's `IsThreat: true` — a silent false positive with no retry. The coordinator's retry mechanism is entirely bypassed whenever a prefilter is present.

The fix is to use `errors.As` to distinguish error types, as the inline comment already acknowledges:

```go
var aErr *AnalysisError
if errors.As(err, &aErr) && aErr.Type == ErrorHard {
    // Hard failure: analysis will never succeed, fall back to prefilter result.
    a.logger.Warn(...)
    metrics.PrefilterOutcome.WithLabelValues(..., "analysis-fallback").Inc()
    return preResult, nil
}
// Soft failure or unknown error: propagate so the coordinator can retry.
return nil, err
```

This behaviour is the security invariant called out in the design review and must be covered by a unit test with an explicit `ErrorSoft` case confirming the error is returned (not swallowed).

---

## Minor

### `internal/config/config.go` — `FrameInterval` comment out of date

```go
FrameInterval time.Duration `mapstructure:"frame_interval"` // default: 2s
```

`config.example.yaml` and the design doc both use `0.25s`. Update the comment to match.
