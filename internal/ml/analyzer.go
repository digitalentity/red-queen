package ml

import (
	"context"

	"redqueen/internal/models"
)

type Result struct {
	IsThreat   bool
	Confidence float64
	Labels     []string
	DetectedAt int64
}

type Analyzer interface {
	Analyze(ctx context.Context, event *models.Event) (*Result, error)
	Name() string
}

type ErrorType int

const (
	ErrorHard ErrorType = iota
	ErrorSoft
)

type AnalysisError struct {
	Type ErrorType
	Err  error
}

func (e *AnalysisError) Error() string { return e.Err.Error() }

func NewAnalysisError(t ErrorType, err error) *AnalysisError {
	return &AnalysisError{Type: t, Err: err}
}
