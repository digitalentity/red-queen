package storage

import (
	"context"
	"sync"

	"redqueen/internal/models"
)

type MockProvider struct {
	mu          sync.Mutex
	SaveFunc    func(ctx context.Context, event *models.Event) (string, error)
	SavedEvents []*models.Event
}

func (m *MockProvider) Type() string {
	return "mock"
}

func (m *MockProvider) Save(ctx context.Context, event *models.Event) (string, error) {
	m.mu.Lock()
	m.SavedEvents = append(m.SavedEvents, event)
	m.mu.Unlock()

	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, event)
	}
	return "http://mock-storage.local/" + event.FilePath, nil
}

func (m *MockProvider) GetSavedEvents() []*models.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to avoid external modification/races
	events := make([]*models.Event, len(m.SavedEvents))
	copy(events, m.SavedEvents)
	return events
}
