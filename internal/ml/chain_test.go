package ml

import (
	"context"
	"errors"
	"testing"
	"time"

	"redqueen/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestEvent() *models.Event {
	return &models.Event{
		Zone:      "test-zone",
		Timestamp: time.Now(),
	}
}

func TestChainedAnalyzer_Analyze(t *testing.T) {
	logger := zap.NewNop()

	t.Run("Prefilter no threat short-circuits", func(t *testing.T) {
		pre := &MockAnalyzer{
			AnalyzeFunc: func(ctx context.Context, e *models.Event) (*Result, error) {
				return &Result{IsThreat: false}, nil
			},
		}
		analysisCalls := 0
		analysis := &MockAnalyzer{
			AnalyzeFunc: func(ctx context.Context, e *models.Event) (*Result, error) {
				analysisCalls++
				return &Result{IsThreat: true}, nil
			},
		}

		chain := NewChainedAnalyzer(pre, analysis, logger)
		res, err := chain.Analyze(context.Background(), newTestEvent())

		require.NoError(t, err)
		assert.False(t, res.IsThreat)
		assert.Equal(t, 0, analysisCalls)
	})

	t.Run("Prefilter threat calls analysis", func(t *testing.T) {
		pre := &MockAnalyzer{
			AnalyzeFunc: func(ctx context.Context, e *models.Event) (*Result, error) {
				return &Result{IsThreat: true, Labels: []string{"person"}}, nil
			},
		}
		analysis := &MockAnalyzer{
			AnalyzeFunc: func(ctx context.Context, e *models.Event) (*Result, error) {
				return &Result{IsThreat: true, Confidence: 0.9, Labels: []string{"person", "weapon"}}, nil
			},
		}

		event := newTestEvent()
		chain := NewChainedAnalyzer(pre, analysis, logger)
		res, err := chain.Analyze(context.Background(), event)

		require.NoError(t, err)
		assert.True(t, res.IsThreat)
		assert.Equal(t, 0.9, res.Confidence)
		assert.Contains(t, res.Labels, "weapon")
		// Verify labels were merged into event
		assert.Contains(t, event.Labels, "person")
	})

	t.Run("Analysis stage hard failure falls back to prefilter result", func(t *testing.T) {
		pre := &MockAnalyzer{
			AnalyzeFunc: func(ctx context.Context, e *models.Event) (*Result, error) {
				return &Result{IsThreat: true, Labels: []string{"person"}}, nil
			},
			NameFunc: func() string { return "yolo" },
		}
		analysis := &MockAnalyzer{
			AnalyzeFunc: func(ctx context.Context, e *models.Event) (*Result, error) {
				return nil, NewAnalysisError(ErrorHard, errors.New("permanent failure"))
			},
			NameFunc: func() string { return "gemini" },
		}

		chain := NewChainedAnalyzer(pre, analysis, logger)
		res, err := chain.Analyze(context.Background(), newTestEvent())

		require.NoError(t, err)
		assert.True(t, res.IsThreat, "Must fall back to prefilter result on hard failure")
		assert.Contains(t, res.Labels, "person")
	})

	t.Run("Analysis stage soft failure propagates error for retry", func(t *testing.T) {
		pre := &MockAnalyzer{
			AnalyzeFunc: func(ctx context.Context, e *models.Event) (*Result, error) {
				return &Result{IsThreat: true, Labels: []string{"person"}}, nil
			},
		}
		analysis := &MockAnalyzer{
			AnalyzeFunc: func(ctx context.Context, e *models.Event) (*Result, error) {
				return nil, NewAnalysisError(ErrorSoft, errors.New("transient failure"))
			},
		}

		chain := NewChainedAnalyzer(pre, analysis, logger)
		res, err := chain.Analyze(context.Background(), newTestEvent())

		require.Error(t, err)
		assert.Nil(t, res)
		var aErr *AnalysisError
		require.True(t, errors.As(err, &aErr))
		assert.Equal(t, ErrorSoft, aErr.Type)
	})
}
