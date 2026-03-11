package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"redqueen/internal/config"
	"redqueen/internal/ml"
	"redqueen/internal/models"
)

const HomeyCloudBaseURL = "https://webhook.homey.app"

type HomeyNotifier struct {
	cfg     config.NotifyConfig
	baseURL string // For testing
	client  *http.Client
}

func NewHomeyNotifier(cfg config.NotifyConfig, client *http.Client) *HomeyNotifier {
	baseURL := HomeyCloudBaseURL
	if cfg.URL != "" {
		baseURL = cfg.URL
	}
	return &HomeyNotifier{
		cfg:     cfg,
		baseURL: baseURL,
		client:  client,
	}
}

func (n *HomeyNotifier) Type() string {
	return "homey"
}

func (n *HomeyNotifier) Condition() string {
	if n.cfg.Condition == "" {
		return "on_threat"
	}
	return n.cfg.Condition
}

func (n *HomeyNotifier) Send(ctx context.Context, event *models.Event, result *ml.Result, artifactURL string) error {
	// Format the message tag
	var message string
	if result.IsThreat {
		message = fmt.Sprintf("Threat detected in %s! Confidence: %.2f. Artifact: %s", 
			event.Zone, result.Confidence, artifactURL)
	} else {
		message = fmt.Sprintf("Event recorded in %s. Confidence: %.2f. Artifact: %s", 
			event.Zone, result.Confidence, artifactURL)
	}

	var u *url.URL
	var err error
	isCloud := n.cfg.HomeyID != "" && n.cfg.URL == ""

	if isCloud {
		// Cloud Webhook: <baseURL>/<HOMEY_ID>/<EVENT_NAME>?tag=<MESSAGE>
		u, err = url.Parse(fmt.Sprintf("%s/%s/%s", n.baseURL, n.cfg.HomeyID, n.cfg.Event))
	} else {
		// Local Homey Pro (2023+): <baseURL>/webhook?event=<EVENT_NAME>&tag=<MESSAGE>
		u, err = url.Parse(fmt.Sprintf("%s/webhook", n.baseURL))
	}
	
	if err != nil {
		return fmt.Errorf("failed to parse Homey webhook URL: %w", err)
	}

	q := u.Query()
	if !isCloud {
		q.Set("event", n.cfg.Event)
	}
	q.Set("tag", message)
	u.RawQuery = q.Encode()


	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create Homey request: %w", err)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("Homey delivery failed: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Homey returned non-200 status: %s", resp.Status)
	}

	return nil
}
