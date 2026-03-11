package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"

	"redqueen/internal/models"
)

// MultiProvider uploads to all configured providers concurrently.
// It returns the URL from the first provider to succeed (in config order).
// It returns an error only if every provider fails.
type MultiProvider struct {
	providers []Provider
	logger    *zap.Logger
}

// NewMultiProvider returns a MultiProvider wrapping the given providers.
// Callers must supply at least two providers.
func NewMultiProvider(providers []Provider, logger *zap.Logger) *MultiProvider {
	return &MultiProvider{providers: providers, logger: logger}
}

// Type implements Provider.
func (m *MultiProvider) Type() string { return "multi" }

// Save uploads the event artifact to all providers concurrently.
func (m *MultiProvider) Save(ctx context.Context, event *models.Event) (string, error) {
	type result struct {
		url string
		err error
		typ string
	}

	results := make([]result, len(m.providers))
	var wg sync.WaitGroup
	for i, p := range m.providers {
		i, p := i, p
		wg.Add(1)
		go func() {
			defer wg.Done()
			url, err := p.Save(ctx, event)
			results[i] = result{url: url, err: err, typ: p.Type()}
		}()
	}
	wg.Wait()

	var firstURL string
	var errs []string
	for _, r := range results {
		if r.err != nil {
			m.logger.Error("Storage provider failed",
				zap.String("type", r.typ), zap.Error(r.err))
			errs = append(errs, fmt.Sprintf("%s: %s", r.typ, r.err))
		} else {
			m.logger.Info("Storage provider succeeded",
				zap.String("type", r.typ), zap.String("url", r.url))
			if firstURL == "" {
				firstURL = r.url
			}
		}
	}

	if firstURL == "" {
		return "", fmt.Errorf("all storage providers failed: %s", strings.Join(errs, "; "))
	}
	return firstURL, nil
}
