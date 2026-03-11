package notify

import (
	"context"

	"redqueen/internal/ml"
	"redqueen/internal/models"
)

type Notifier interface {
	Send(ctx context.Context, event *models.Event, result *ml.Result, artifactURL string) error
	Type() string
}
