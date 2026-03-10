package ml

import (
	"context"
	"time"

	"redqueen/internal/models"
)

type MockAnalyzer struct {
	AnalyzeFunc func(ctx context.Context, event *models.Event) (*Result, error)
}

func (m *MockAnalyzer) Analyze(ctx context.Context, event *models.Event) (*Result, error) {
	if m.AnalyzeFunc != nil {
		return m.AnalyzeFunc(ctx, event)
	}
	// Default: No threat
	return &Result{
		IsThreat:   false,
		Confidence: 0.99,
		Labels:     []string{"clear"},
		DetectedAt: time.Now().Unix(),
	}, nil
}
