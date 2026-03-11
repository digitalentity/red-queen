package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"redqueen/internal/config"
	"redqueen/internal/ml"
	"redqueen/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHomeyNotifier_Send(t *testing.T) {
	// Setup test data
	event := &models.Event{
		Zone: "FrontDoor",
	}
	result := &ml.Result{
		Confidence: 0.95,
	}
	artifactURL := "/artifacts/test.jpg"

	// Mock Homey Webhook server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/my-homey-id/alert-event", r.URL.Path)
		
		tag := r.URL.Query().Get("tag")
		assert.Contains(t, tag, "Threat detected in FrontDoor!")
		assert.Contains(t, tag, "Confidence: 0.95")
		assert.Contains(t, tag, "Artifact: /artifacts/test.jpg")
		
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Setup notifier with mock base URL
	cfg := config.NotifyConfig{
		HomeyID: "my-homey-id",
		Event:   "alert-event",
	}
	notifier := NewHomeyNotifier(cfg, http.DefaultClient)
	notifier.baseURL = ts.URL

	// Send notification
	err := notifier.Send(context.Background(), event, result, artifactURL)
	require.NoError(t, err)
}

func TestHomeyNotifier_SendLocal(t *testing.T) {
	// Setup test data
	event := &models.Event{
		Zone: "Garage",
	}
	result := &ml.Result{
		Confidence: 0.88,
	}
	artifactURL := "/artifacts/local.jpg"

	// Mock Homey Local server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/webhook", r.URL.Path)
		
		assert.Equal(t, "threat-detected", r.URL.Query().Get("event"))
		tag := r.URL.Query().Get("tag")
		assert.Contains(t, tag, "Threat detected in Garage!")
		
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Setup notifier with local URL
	cfg := config.NotifyConfig{
		URL:   ts.URL,
		Event: "threat-detected",
	}
	notifier := NewHomeyNotifier(cfg, http.DefaultClient)

	// Send notification
	err := notifier.Send(context.Background(), event, result, artifactURL)
	require.NoError(t, err)
}
