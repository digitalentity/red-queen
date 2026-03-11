package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"redqueen/internal/config"
	"redqueen/internal/ml"
	"redqueen/internal/models"
)

type TelegramNotifier struct {
	cfg    config.NotifyConfig
	apiURL string // Added for testing
	client *http.Client
}

func NewTelegramNotifier(cfg config.NotifyConfig, client *http.Client) *TelegramNotifier {
	apiURL := "https://api.telegram.org"
	if cfg.URL != "" {
		apiURL = cfg.URL
	}
	return &TelegramNotifier{
		cfg:    cfg,
		apiURL: apiURL,
		client: client,
	}
}

func (n *TelegramNotifier) Type() string {
	return "telegram"
}

func (n *TelegramNotifier) Send(ctx context.Context, event *models.Event, result *ml.Result, artifactURL string) error {
	message := n.formatMessage(event, result, artifactURL)

	// Attempt to send media first
	err := n.sendMedia(ctx, event, message)
	if err == nil {
		return nil
	}

	// Fallback to plain text message if media fails
	return n.sendMessage(ctx, message)
}

func (n *TelegramNotifier) formatMessage(event *models.Event, result *ml.Result, artifactURL string) string {
	var sb strings.Builder
	sb.WriteString("🚨 *Threat Detected!*\n\n")
	sb.WriteString(fmt.Sprintf("*Zone:* %s\n", n.escapeMarkdown(event.Zone)))
	sb.WriteString(fmt.Sprintf("*Confidence:* %.0f%%\n", result.Confidence*100))
	
	if len(result.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("*Objects:* %s\n", n.escapeMarkdown(strings.Join(result.Labels, ", "))))
	}
	
	sb.WriteString(fmt.Sprintf("*Time:* %s\n", n.escapeMarkdown(event.Timestamp.Format("2006-01-02 15:04:05"))))

	if n.cfg.URL != "" && artifactURL != "" {
		fullURL := strings.TrimSuffix(n.cfg.URL, "/") + artifactURL
		sb.WriteString(fmt.Sprintf("\n[View Full Resolution](%s)", fullURL))
	}

	return sb.String()
}

func (n *TelegramNotifier) sendMedia(ctx context.Context, event *models.Event, caption string) error {
	file, err := os.Open(event.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open artifact for Telegram: %w", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(event.FilePath))
	method := "sendPhoto"
	fieldName := "photo"
	
	if ext == ".mp4" || ext == ".mov" || ext == ".avi" {
		method = "sendVideo"
		fieldName = "video"
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add Chat ID
	if err := writer.WriteField("chat_id", fmt.Sprintf("%d", n.cfg.ChatID)); err != nil {
		return err
	}

	// Add Caption
	if err := writer.WriteField("caption", n.truncate(caption, 1024)); err != nil {
		return err
	}

	// Add Parse Mode
	if err := writer.WriteField("parse_mode", "MarkdownV2"); err != nil {
		return err
	}

	// Add File
	part, err := writer.CreateFormFile(fieldName, filepath.Base(event.FilePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	url := fmt.Sprintf("%s/bot%s/%s", n.apiURL, n.cfg.Token, method)
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error (%s): %s", method, string(bodyBytes))
	}

	return nil
}

func (n *TelegramNotifier) sendMessage(ctx context.Context, text string) error {
	payload := map[string]interface{}{
		"chat_id":    n.cfg.ChatID,
		"text":       n.truncate(text, 4096),
		"parse_mode": "MarkdownV2",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", n.apiURL, n.cfg.Token)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error (sendMessage): %s", string(bodyBytes))
	}

	return nil
}

// escapeMarkdown handles Telegram MarkdownV2 special characters.
func (n *TelegramNotifier) escapeMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]", "(", "\\(", ")", "\\)", "~", "\\~", "`", "\\`", ">", "\\>", "#", "\\#", "+", "\\+", "-", "\\-", "=", "\\=", "|", "\\|", "{", "\\{", "}", "\\}", ".", "\\.", "!", "\\!",
	)
	return replacer.Replace(text)
}

func (n *TelegramNotifier) truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit-3] + "..."
}
