package ml

import (
	"context"
	"os"
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

	// For testing purposes, allow triggering a threat via env var
	isThreat := os.Getenv("RED_QUEEN_MOCK_THREAT") == "true"
	labels := []string{"clear"}
	if isThreat {
		labels = []string{"person", "weapon"}
	}

	return &Result{
		IsThreat:   isThreat,
		Confidence: 0.99,
		Labels:     labels,
		DetectedAt: time.Now().Unix(),
	}, nil
}
