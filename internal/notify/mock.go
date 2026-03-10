package notify

import (
	"context"

	"redqueen/internal/ml"
	"redqueen/internal/models"
)

type SentAlert struct {
	Event       *models.Event
	Result      *ml.Result
	ArtifactURL string
}

type MockNotifier struct {
	SendFunc   func(ctx context.Context, event *models.Event, result *ml.Result, artifactURL string) error
	SentAlerts []SentAlert
}

func (m *MockNotifier) Send(ctx context.Context, event *models.Event, result *ml.Result, artifactURL string) error {
	m.SentAlerts = append(m.SentAlerts, SentAlert{Event: event, Result: result, ArtifactURL: artifactURL})
	if m.SendFunc != nil {
		return m.SendFunc(ctx, event, result, artifactURL)
	}
	return nil
}
