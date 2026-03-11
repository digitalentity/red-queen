package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"redqueen/internal/config"
	"redqueen/internal/ml"
	"redqueen/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestTelegramNotifier_formatMessage(t *testing.T) {
	n := &TelegramNotifier{
		cfg: config.NotifyConfig{
			URL: "https://redqueen.io/",
		},
	}

	event := &models.Event{
		Zone:      "Front Door",
		Timestamp: time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC),
	}
	result := &ml.Result{
		Confidence: 0.956,
		Labels:     []string{"Person", "Backpack"},
	}
	artifactURL := "/artifacts/2026-03-10/Front%20Door/event1.jpg"

	msg := n.formatMessage(event, result, artifactURL)

	assert.Contains(t, msg, "🚨 *Threat Detected!*")
	assert.Contains(t, msg, "*Zone:* Front Door")
	assert.Contains(t, msg, "*Confidence:* 96%")
	assert.Contains(t, msg, "*Objects:* Person, Backpack")
	assert.Contains(t, msg, "[View Full Resolution](https://redqueen.io/artifacts/2026-03-10/Front%20Door/event1.jpg)")
}

func TestTelegramNotifier_Send_Fallback(t *testing.T) {
	// 1. Create a dummy file
	tmpFile, err := os.CreateTemp("", "artifact-*.jpg")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 2. Mock Telegram Server (Fail media, succeed text)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/botTEST_TOKEN/sendPhoto" {
			w.WriteHeader(http.StatusBadRequest) // Simulate media upload failure
			w.Write([]byte(`{"ok": false, "description": "some error"}`))
			return
		}
		if r.URL.Path == "/botTEST_TOKEN/sendMessage" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok": true}`))
			return
		}
	}))
	defer server.Close()

	// 3. Configure Notifier
	n := &TelegramNotifier{
		cfg: config.NotifyConfig{
			Token:  "TEST_TOKEN",
			ChatID: 12345,
		},
		apiURL: server.URL,
		client: http.DefaultClient,
	}

	event := &models.Event{
		FilePath: tmpFile.Name(),
		Zone:     "Garden",
	}
	result := &ml.Result{Confidence: 0.9}

	err = n.Send(context.Background(), event, result, "")
	assert.NoError(t, err)
}

func TestTelegramNotifier_Send_Success(t *testing.T) {
	// 1. Create a dummy file
	tmpFile, err := os.CreateTemp("", "artifact-*.jpg")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 2. Mock Telegram Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/botTEST_TOKEN/sendPhoto", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	// 3. Configure Notifier
	n := &TelegramNotifier{
		cfg: config.NotifyConfig{
			Token:  "TEST_TOKEN",
			ChatID: 12345,
		},
		apiURL: server.URL,
		client: http.DefaultClient,
	}

	event := &models.Event{
		FilePath: tmpFile.Name(),
		Zone:     "Garden",
	}
	result := &ml.Result{Confidence: 0.9}

	err = n.Send(context.Background(), event, result, "")
	assert.NoError(t, err)
}
