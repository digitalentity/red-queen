package ml

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"redqueen/internal/config"
	"redqueen/internal/models"

	"google.golang.org/genai"
	"go.uber.org/zap"
)

type VertexAnalyzer struct {
	logger *zap.Logger
	cfg    config.MLConfig
	client *genai.Client
}

func NewVertexAnalyzer(ctx context.Context, logger *zap.Logger, cfg config.MLConfig) (*VertexAnalyzer, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  cfg.ProjectID,
		Location: cfg.Location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create vertex ai client: %w", err)
	}

	return &VertexAnalyzer{
		logger: logger,
		cfg:    cfg,
		client: client,
	}, nil
}

type vertexResponse struct {
	IsThreat    bool     `json:"is_threat"`
	Confidence  float64  `json:"confidence"`
	Labels      []string `json:"labels"`
	Description string   `json:"description"`
}

func (a *VertexAnalyzer) Analyze(ctx context.Context, event *models.Event) (*Result, error) {
	// 1. Check file size before reading to avoid OOM
	fileInfo, err := os.Stat(event.FilePath)
	if err != nil {
		return nil, NewAnalysisError(ErrorHard, fmt.Errorf("failed to stat file: %w", err))
	}

	maxSize := int64(20 * 1024 * 1024) // 20 MB default safety limit
	if a.cfg.MaxArtifactSize > 0 {
		maxSize = a.cfg.MaxArtifactSize
	}

	if fileInfo.Size() > maxSize {
		return nil, NewAnalysisError(ErrorHard, fmt.Errorf("file size %d exceeds maximum allowed size %d", fileInfo.Size(), maxSize))
	}

	// 2. Detect MIME Type
	mimeType := a.detectMIMEType(event.FilePath)
	a.logger.Debug("Detected artifact type", zap.String("mime_type", mimeType), zap.String("path", event.FilePath))

	// 3. Read file into memory (Inline Data)
	data, err := os.ReadFile(event.FilePath)
	if err != nil {
		return nil, NewAnalysisError(ErrorHard, fmt.Errorf("failed to read file: %w", err))
	}

	// 4. Generate Content
	prompt := fmt.Sprintf("Analyze this artifact from zone '%s'. Detect any of these objects: %s.", 
		event.Zone, strings.Join(a.cfg.TargetObjects, ", "))

	systemInstruction := fmt.Sprintf(`You are a security analyst. 
Analyze the provided media for security threats in the zone: %s.
Search specifically for: %s.
You MUST return a valid JSON object with the following structure:
{
  "is_threat": boolean,
  "confidence": float (0.0 to 1.0),
  "labels": [string],
  "description": string
}`, event.Zone, strings.Join(a.cfg.TargetObjects, ", "))

	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{
					InlineData: &genai.Blob{
						MIMEType: mimeType,
						Data:     data,
					},
				},
				{
					Text: prompt,
				},
			},
		},
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{
					Text: systemInstruction,
				},
			},
		},
		ResponseMIMEType: "application/json",
	}

	res, err := a.client.Models.GenerateContent(ctx, a.cfg.ModelName, contents, config)
	if err != nil {
		return nil, NewAnalysisError(ErrorSoft, fmt.Errorf("vertex ai generation failed: %w", err))
	}

	if res == nil || len(res.Candidates) == 0 {
		return nil, NewAnalysisError(ErrorSoft, fmt.Errorf("no response candidates from vertex ai"))
	}

	// Extract text from response safely
	text := res.Text()
	if text == "" {
		return nil, NewAnalysisError(ErrorSoft, fmt.Errorf("empty response text from vertex ai"))
	}

	// Extract JSON from response
	var vResp vertexResponse
	if err := json.Unmarshal([]byte(text), &vResp); err != nil {
		a.logger.Error("Failed to parse vertex ai JSON response", zap.Error(err), zap.String("raw", text))
		return nil, NewAnalysisError(ErrorHard, fmt.Errorf("invalid JSON from model: %w", err))
	}

	// Adjust threat status based on threshold if model says it's a threat but confidence is low
	isThreat := vResp.IsThreat && vResp.Confidence >= a.cfg.Threshold

	return &Result{
		IsThreat:   isThreat,
		Confidence: vResp.Confidence,
		Labels:     vResp.Labels,
		DetectedAt: event.Timestamp.Unix(),
	}, nil
}

func (a *VertexAnalyzer) detectMIMEType(filePath string) string {
	// First, try to detect by sniffing the first 512 bytes
	f, err := os.Open(filePath)
	if err == nil {
		defer f.Close()
		buffer := make([]byte, 512)
		n, err := f.Read(buffer)
		if err == nil && n > 0 {
			contentType := http.DetectContentType(buffer[:n])
			if contentType != "application/octet-stream" {
				return contentType
			}
		}
	}

	// Fallback to extension
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	default:
		return "video/mp4" // Default assumption
	}
}

func (a *VertexAnalyzer) Name() string {
	return "vertex-ai"
}
