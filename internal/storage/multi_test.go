package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"redqueen/internal/models"
)

// stubProvider is a minimal Provider for MultiProvider tests.
type stubProvider struct {
	typ     string
	url     string
	err     error
	saveCalled bool
}

func (s *stubProvider) Type() string { return s.typ }
func (s *stubProvider) Save(_ context.Context, _ *models.Event) (string, error) {
	s.saveCalled = true
	return s.url, s.err
}

var testEvent = &models.Event{
	ID:        "evt-multi",
	FilePath:  "/tmp/clip.mp4",
	Zone:      "Zone1",
	Timestamp: time.Now(),
}

func TestMultiProvider_Type(t *testing.T) {
	m := NewMultiProvider(nil, zap.NewNop())
	assert.Equal(t, "multi", m.Type())
}

func TestMultiProvider_AllSucceed(t *testing.T) {
	a := &stubProvider{typ: "a", url: "http://a/artifact"}
	b := &stubProvider{typ: "b", url: "http://b/artifact"}
	m := NewMultiProvider([]Provider{a, b}, zap.NewNop())

	url, err := m.Save(context.Background(), testEvent)

	require.NoError(t, err)
	// First provider's URL is returned.
	assert.Equal(t, "http://a/artifact", url)
	assert.True(t, a.saveCalled)
	assert.True(t, b.saveCalled)
}

func TestMultiProvider_FirstFails_SecondSucceeds(t *testing.T) {
	a := &stubProvider{typ: "a", err: errors.New("a failed")}
	b := &stubProvider{typ: "b", url: "http://b/artifact"}
	m := NewMultiProvider([]Provider{a, b}, zap.NewNop())

	url, err := m.Save(context.Background(), testEvent)

	require.NoError(t, err)
	assert.Equal(t, "http://b/artifact", url)
}

func TestMultiProvider_SecondFails_FirstSucceeds(t *testing.T) {
	a := &stubProvider{typ: "a", url: "http://a/artifact"}
	b := &stubProvider{typ: "b", err: errors.New("b failed")}
	m := NewMultiProvider([]Provider{a, b}, zap.NewNop())

	url, err := m.Save(context.Background(), testEvent)

	require.NoError(t, err)
	// First provider succeeded; its URL is returned despite b failing.
	assert.Equal(t, "http://a/artifact", url)
}

func TestMultiProvider_AllFail(t *testing.T) {
	a := &stubProvider{typ: "a", err: errors.New("a failed")}
	b := &stubProvider{typ: "b", err: errors.New("b failed")}
	m := NewMultiProvider([]Provider{a, b}, zap.NewNop())

	_, err := m.Save(context.Background(), testEvent)

	require.Error(t, err)
	assert.ErrorContains(t, err, "all storage providers failed")
	assert.ErrorContains(t, err, "a failed")
	assert.ErrorContains(t, err, "b failed")
}

func TestMultiProvider_AllProvidersCalledRegardlessOfFailure(t *testing.T) {
	a := &stubProvider{typ: "a", err: errors.New("a failed")}
	b := &stubProvider{typ: "b", url: "http://b/artifact"}
	c := &stubProvider{typ: "c", url: "http://c/artifact"}
	m := NewMultiProvider([]Provider{a, b, c}, zap.NewNop())

	_, err := m.Save(context.Background(), testEvent)

	require.NoError(t, err)
	assert.True(t, a.saveCalled, "a should have been called")
	assert.True(t, b.saveCalled, "b should have been called")
	assert.True(t, c.saveCalled, "c should have been called")
}
