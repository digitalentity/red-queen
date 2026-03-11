package ml

import (
	"context"
	"errors"
	"fmt"
	"time"

	"redqueen/internal/metrics"
	"redqueen/internal/models"

	"go.uber.org/zap"
)

type ChainedAnalyzer struct {
	prefilter Analyzer
	analysis  Analyzer
	logger    *zap.Logger
}

func NewChainedAnalyzer(prefilter, analysis Analyzer, logger *zap.Logger) *ChainedAnalyzer {
	return &ChainedAnalyzer{
		prefilter: prefilter,
		analysis:  analysis,
		logger:    logger,
	}
}

func (a *ChainedAnalyzer) Analyze(ctx context.Context, event *models.Event) (*Result, error) {
	startTime := time.Now()

	// 1. Run Prefilter
	preResult, err := a.prefilter.Analyze(ctx, event)

	// Record prefilter metrics
	duration := time.Since(startTime).Seconds()
	metrics.PrefilterDuration.WithLabelValues(event.Zone, a.prefilter.Name()).Observe(duration)

	if err != nil {
		metrics.PrefilterOutcome.WithLabelValues(event.Zone, a.prefilter.Name(), "error").Inc()
		return nil, err
	}

	// 2. Short-circuit if prefilter doesn't see a threat
	if !preResult.IsThreat {
		metrics.PrefilterOutcome.WithLabelValues(event.Zone, a.prefilter.Name(), "filtered").Inc()
		return preResult, nil
	}

	metrics.PrefilterOutcome.WithLabelValues(event.Zone, a.prefilter.Name(), "pass").Inc()

	// 3. Append prefilter labels to the event for context in the next stage
	event.Labels = append(event.Labels, preResult.Labels...)

	// 4. Run Analysis Stage
	analysisResult, err := a.analysis.Analyze(ctx, event)
	if err != nil {
		var aErr *AnalysisError
		// Fail-secure: if analysis stage hard-fails but prefilter saw a threat, return prefilter result.
		if errors.As(err, &aErr) && aErr.Type == ErrorHard {
			a.logger.Warn("Analysis stage hard-failed, falling back to prefilter result",
				zap.Error(err),
				zap.String("prefilter", a.prefilter.Name()),
				zap.String("analysis", a.analysis.Name()))

			metrics.PrefilterOutcome.WithLabelValues(event.Zone, a.prefilter.Name(), "analysis-fallback").Inc()
			return preResult, nil
		}

		// Soft failure or unknown error: propagate so the coordinator can retry.
		return nil, err
	}

	return analysisResult, nil
}

func (a *ChainedAnalyzer) Name() string {
	return fmt.Sprintf("%s+%s", a.prefilter.Name(), a.analysis.Name())
}
