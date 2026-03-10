package storage

import (
	"context"

	"redqueen/internal/models"
)

type MockProvider struct {
	SaveFunc    func(ctx context.Context, event *models.Event) (string, error)
	SavedEvents []*models.Event
}

func (m *MockProvider) Save(ctx context.Context, event *models.Event) (string, error) {
	m.SavedEvents = append(m.SavedEvents, event)
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, event)
	}
	return "http://mock-storage.local/" + event.FilePath, nil
}
