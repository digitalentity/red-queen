package ml

import (
	"context"
	"fmt"

	"redqueen/internal/config"

	"go.uber.org/zap"
)

// NewAnalyzer constructs an Analyzer from the given config.
func NewAnalyzer(ctx context.Context, cfg config.AnalyzerConfig, logger *zap.Logger) (Analyzer, error) {
	switch cfg.Provider {
	case "yolo-onnx":
		// TODO: Implement YOLOAnalyzer
		return nil, fmt.Errorf("yolo-onnx provider not yet implemented")
	case "gemini-ai":
		return NewGeminiAnalyzer(ctx, cfg, logger)
	case "always":
		return &PassThroughAnalyzer{}, nil
	case "mock":
		return &MockAnalyzer{}, nil
	default:
		logger.Warn("Unknown or no analyzer provider configured, using mock", zap.String("provider", cfg.Provider))
		return &MockAnalyzer{}, nil
	}
}
