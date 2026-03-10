package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"redqueen/internal/config"
	"redqueen/internal/ml"
	"redqueen/internal/models"
)

type WebhookNotifier struct {
	cfg config.NotifyConfig
}

func NewWebhookNotifier(cfg config.NotifyConfig) *WebhookNotifier {
	return &WebhookNotifier{
		cfg: cfg,
	}
}

type WebhookPayload struct {
	EventID     string    `json:"event_id"`
	Timestamp   time.Time `json:"timestamp"`
	Zone        string    `json:"zone"`
	CameraIP    string    `json:"camera_ip"`
	IsThreat    bool      `json:"is_threat"`
	Confidence  float64   `json:"confidence"`
	Labels      []string  `json:"labels"`
	ArtifactURL string    `json:"artifact_url"`
}

func (n *WebhookNotifier) Send(ctx context.Context, event *models.Event, result *ml.Result, artifactURL string) error {
	payload := WebhookPayload{
		EventID:     event.ID,
		Timestamp:   event.Timestamp,
		Zone:        event.Zone,
		CameraIP:    event.CameraIP,
		IsThreat:    result.IsThreat,
		Confidence:  result.Confidence,
		Labels:      result.Labels,
		ArtifactURL: artifactURL,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", n.cfg.URL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "RedQueen/1.0")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook delivery failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-success status: %s", resp.Status)
	}

	return nil
}
