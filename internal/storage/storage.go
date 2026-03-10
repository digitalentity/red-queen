package storage

import (
	"context"

	"redqueen/internal/models"
)

type Provider interface {
	Save(ctx context.Context, event *models.Event) (string, error)
}
