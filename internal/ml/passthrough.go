package ml

import (
	"context"
	"time"

	"redqueen/internal/models"
)

// PassThroughAnalyzer always returns a threat result.
// Useful for debugging end-to-end integration (Storage, Notifications).
type PassThroughAnalyzer struct{}

func (a *PassThroughAnalyzer) Analyze(ctx context.Context, event *models.Event) (*Result, error) {
	return &Result{
		IsThreat:   true,
		Confidence: 1.0,
		Labels:     []string{"debug-passthrough"},
		DetectedAt: time.Now().Unix(),
	}, nil
}

func (a *PassThroughAnalyzer) Name() string {
	return "passthrough"
}
