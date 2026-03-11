package notify

import (
	"context"
	"sync"

	"redqueen/internal/ml"
	"redqueen/internal/models"
)

type SentAlert struct {
	Event       *models.Event
	Result      *ml.Result
	ArtifactURL string
}

type MockNotifier struct {
	mu         sync.Mutex
	SendFunc   func(ctx context.Context, event *models.Event, result *ml.Result, artifactURL string) error
	SentAlerts []SentAlert
	MockCondition string
}

func (m *MockNotifier) Type() string {
	return "mock"
}

func (m *MockNotifier) Condition() string {
	if m.MockCondition == "" {
		return "on_threat"
	}
	return m.MockCondition
}

func (m *MockNotifier) Send(ctx context.Context, event *models.Event, result *ml.Result, artifactURL string) error {
	m.mu.Lock()
	m.SentAlerts = append(m.SentAlerts, SentAlert{Event: event, Result: result, ArtifactURL: artifactURL})
	m.mu.Unlock()

	if m.SendFunc != nil {
		return m.SendFunc(ctx, event, result, artifactURL)
	}
	return nil
}

func (m *MockNotifier) GetSentAlerts() []SentAlert {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to avoid external modification/races
	alerts := make([]SentAlert, len(m.SentAlerts))
	copy(alerts, m.SentAlerts)
	return alerts
}
