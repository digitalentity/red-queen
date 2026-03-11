package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"redqueen/internal/config"
	"redqueen/internal/ml"
	"redqueen/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookNotifier_Send(t *testing.T) {
	// Setup test server
	var receivedPayload WebhookPayload
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		
		err := json.NewDecoder(r.Body).Decode(&receivedPayload)
		require.NoError(t, err)
		
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Setup notifier
	cfg := config.NotifyConfig{
		Type:    "webhook",
		Enabled: true,
		URL:     ts.URL,
	}
	notifier := NewWebhookNotifier(cfg, http.DefaultClient)

	// Setup test data
	event := &models.Event{
		ID:        "test-event",
		Timestamp: time.Now().Truncate(time.Second),
		Zone:      "test-zone",
		CameraIP:  "1.2.3.4",
	}
	result := &ml.Result{
		IsThreat:   true,
		Confidence: 0.99,
		Labels:     []string{"person"},
	}
	artifactURL := "/artifacts/test.jpg"

	// Send notification
	err := notifier.Send(context.Background(), event, result, artifactURL)
	require.NoError(t, err)

	// Verify payload
	assert.Equal(t, event.ID, receivedPayload.EventID)
	assert.True(t, event.Timestamp.Equal(receivedPayload.Timestamp))
	assert.Equal(t, event.Zone, receivedPayload.Zone)
	assert.Equal(t, event.CameraIP, receivedPayload.CameraIP)
	assert.Equal(t, result.IsThreat, receivedPayload.IsThreat)
	assert.Equal(t, result.Confidence, receivedPayload.Confidence)
	assert.Equal(t, result.Labels, receivedPayload.Labels)
	assert.Equal(t, artifactURL, receivedPayload.ArtifactURL)
}

func TestWebhookNotifier_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	notifier := NewWebhookNotifier(config.NotifyConfig{URL: ts.URL}, http.DefaultClient)
	err := notifier.Send(context.Background(), &models.Event{}, &ml.Result{}, "")
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500 Internal Server Error")
}
