package ml

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	data, err := os.ReadFile(event.FilePath)
	if err != nil {
		return nil, NewAnalysisError(ErrorHard, fmt.Errorf("failed to read video file: %w", err))
	}

	// Dynamic prompt adjustment based on zone
	prompt := fmt.Sprintf("Analyze this video from zone '%s'. Detect any of these objects: %s.", 
		event.Zone, strings.Join(a.cfg.TargetObjects, ", "))

	systemInstruction := fmt.Sprintf(`You are a security video analyst. 
Analyze the provided video clip for security threats in the zone: %s.
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
						MIMEType: "video/mp4",
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

	// Extract text from response
	text := res.Text()
	if text == "" {
		return nil, NewAnalysisError(ErrorSoft, fmt.Errorf("empty response from vertex ai"))
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
